package git

type GraphEdge struct {
	Parent string
	Column int
}

type GraphRow struct {
	Column   int
	Incoming []int
	Through  []int
	Edges    []GraphEdge
	Width    int
}

func Lanes(commits []Commit) []GraphRow {
	rows := make([]GraphRow, len(commits))
	var lanes []string
	for i, c := range commits {
		width := len(lanes)
		var incoming, through []int
		for k, id := range lanes {
			switch {
			case id == c.OID:
				incoming = append(incoming, k)
			case id != "":
				through = append(through, k)
			}
		}
		column := firstFreeLane(lanes)
		if len(incoming) > 0 {
			column = incoming[0]
		}
		for _, k := range incoming {
			lanes[k] = ""
		}
		var edges []GraphEdge
		for pi, parent := range c.Parents {
			target := laneOf(lanes, parent)
			if target < 0 {
				target = column
				if pi > 0 {
					target = firstFreeLane(lanes)
				}
				for len(lanes) <= target {
					lanes = append(lanes, "")
				}
				lanes[target] = parent
			}
			edges = append(edges, GraphEdge{Parent: parent, Column: target})
		}
		for len(lanes) > 0 && lanes[len(lanes)-1] == "" {
			lanes = lanes[:len(lanes)-1]
		}
		width = max(width, len(lanes), column+1)
		rows[i] = GraphRow{Column: column, Incoming: incoming, Through: through, Edges: edges, Width: width}
	}
	return rows
}

func firstFreeLane(lanes []string) int {
	for k, id := range lanes {
		if id == "" {
			return k
		}
	}
	return len(lanes)
}

func laneOf(lanes []string, oid string) int {
	for k, id := range lanes {
		if id == oid {
			return k
		}
	}
	return -1
}
