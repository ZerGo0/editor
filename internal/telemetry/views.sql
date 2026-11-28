-- DuckDB views over the telemetry files that the app writes to
-- ~/Library/Logs/Editor/telemetry/ (Editor Dev for the dev profile).
-- Usage:
--   duckdb -cmd "SET VARIABLE telemetry_dir = '$HOME/Library/Logs/Editor/telemetry'" \
--     -cmd ".read internal/telemetry/views.sql" -c "FROM histograms"

CREATE OR REPLACE TYPE otel_kv AS STRUCT(Key VARCHAR, Value STRUCT(Type VARCHAR, Value JSON));

CREATE OR REPLACE TYPE otel_point AS STRUCT(
    Attributes STRUCT(Key VARCHAR, Value STRUCT(Type VARCHAR, Value JSON))[], Time VARCHAR, Count BIGINT, Sum DOUBLE, Min DOUBLE, Max DOUBLE,
    Bounds DOUBLE[], BucketCounts BIGINT[], Value DOUBLE
);

CREATE OR REPLACE MACRO otel_text(v) AS
    CASE WHEN typeof(v) = 'JSON' THEN CAST(v AS JSON) ->> '$' ELSE CAST(v AS VARCHAR) END;

CREATE OR REPLACE MACRO otel_attrs(kvs) AS
    map_from_entries(list_transform(coalesce(kvs, []), lambda kv: struct_pack(k := kv.Key, v := otel_text(kv.Value.Value))));

CREATE OR REPLACE MACRO hist_bound(bounds, counts, max_value, q) AS coalesce(
    bounds[list_filter(
        range(1, len(counts) + 1),
        lambda i: list_sum(list_slice(counts, 1, i)) >= q * list_sum(counts)
    )[1]],
    max_value
);

CREATE OR REPLACE VIEW spans AS
SELECT
    otel_attrs(Resource)['session.id'] AS session,
    otel_attrs(Resource)['service.version'] AS version,
    Name AS name,
    CAST(StartTime AS TIMESTAMPTZ) AS started,
    epoch_us(CAST(EndTime AS TIMESTAMPTZ)) / 1000.0 - epoch_us(CAST(StartTime AS TIMESTAMPTZ)) / 1000.0 AS duration_ms,
    otel_attrs(Attributes) AS attributes
FROM read_json(getvariable('telemetry_dir') || '/traces*.jsonl', format = 'newline_delimited', maximum_object_size = 16777216, columns = {
    Name: 'VARCHAR', StartTime: 'VARCHAR', EndTime: 'VARCHAR', Attributes: 'otel_kv[]', Resource: 'otel_kv[]'
});

CREATE OR REPLACE VIEW budget_events AS
SELECT
    otel_attrs(Resource)['session.id'] AS session,
    CAST(Timestamp AS TIMESTAMPTZ) AS logged_at,
    otel_attrs(Attributes)['action'] AS action,
    otel_attrs(Attributes)['span'] AS span,
    CAST(otel_attrs(Attributes)['value'] AS DOUBLE) AS value,
    CAST(otel_attrs(Attributes)['budget'] AS DOUBLE) AS budget,
    otel_attrs(Attributes)['unit'] AS unit
FROM read_json(getvariable('telemetry_dir') || '/logs*.jsonl', format = 'newline_delimited', maximum_object_size = 16777216, columns = {
    Timestamp: 'VARCHAR', EventName: 'VARCHAR', Attributes: 'otel_kv[]', Resource: 'otel_kv[]'
})
WHERE EventName = 'budget.exceeded';

CREATE OR REPLACE VIEW metric_points AS
WITH exports AS (
    SELECT otel_attrs(Resource)['session.id'] AS session, filename, row_number() OVER (PARTITION BY filename) AS export, unnest(ScopeMetrics) AS sm
    FROM read_json(getvariable('telemetry_dir') || '/metrics*.jsonl', format = 'newline_delimited', maximum_object_size = 16777216, filename = true, columns = {
        Resource: 'otel_kv[]',
        ScopeMetrics: 'STRUCT(Metrics STRUCT(Name VARCHAR, Unit VARCHAR, Data STRUCT(DataPoints otel_point[]))[])[]'
    })
), metrics AS (
    SELECT session, filename, export, unnest(sm.Metrics) AS m FROM exports
), points AS (
    SELECT session, filename, export, m.Name AS metric, m.Unit AS unit, unnest(m.Data.DataPoints) AS p FROM metrics
)
SELECT
    session,
    filename || '#' || export AS export,
    metric,
    unit,
    CAST(p.Time AS TIMESTAMPTZ) AS collected_at,
    otel_attrs(p.Attributes) AS attributes,
    p.Count AS count,
    p.Sum AS sum,
    p.Min AS min,
    p.Max AS max,
    p.Bounds AS bounds,
    p.BucketCounts AS bucket_counts,
    CAST(p.Value AS DOUBLE) AS value
FROM points;

CREATE OR REPLACE VIEW latest_points AS
SELECT * FROM metric_points
QUALIFY row_number() OVER (PARTITION BY session, metric, attributes ORDER BY collected_at DESC) = 1;

CREATE OR REPLACE VIEW histograms AS
SELECT
    session,
    metric,
    coalesce(attributes['span'], '') AS span,
    count,
    round(sum / nullif(count, 0), 2) AS avg_ms,
    round(max, 2) AS max_ms,
    hist_bound(bounds, bucket_counts, max, 0.5) AS p50_ms,
    hist_bound(bounds, bucket_counts, max, 0.95) AS p95_ms
FROM latest_points
WHERE count IS NOT NULL;

CREATE OR REPLACE VIEW process AS
SELECT session, min(collected_at) AS collected_at,
    max(value) FILTER (WHERE metric = 'editor.process.cpu.time') AS cpu_seconds,
    round(max(value) FILTER (WHERE metric = 'editor.process.memory.peak_rss') / 1048576, 1) AS peak_rss_mib,
    round(max(value) FILTER (WHERE metric = 'editor.go.heap') / 1048576, 1) AS heap_mib,
    round(max(value) FILTER (WHERE metric = 'editor.disk.written') / 1048576, 2) AS written_mib
FROM metric_points
WHERE metric IN ('editor.process.cpu.time', 'editor.process.memory.peak_rss', 'editor.go.heap', 'editor.disk.written')
GROUP BY session, export
ORDER BY session, collected_at;
