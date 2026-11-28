package review

import (
	"encoding/json"
	"strings"
)

const planKey = "plan"

type reviewJSON struct {
	Decision Decision `json:"decision"`
	Message  string   `json:"message"`
}

type annotateJSON struct {
	Decision        Decision `json:"decision"`
	Feedback        *string  `json:"feedback,omitempty"`
	AnnotationCount *int     `json:"annotationCount,omitempty"`
	Target          string   `json:"target,omitempty"`
}

type hookBlock struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

type permissionOutput struct {
	HookSpecificOutput permissionDecision `json:"hookSpecificOutput"`
}

type permissionDecision struct {
	HookEventName string     `json:"hookEventName"`
	Decision      permission `json:"decision"`
}

type permission struct {
	Behavior     string                     `json:"behavior"`
	UpdatedInput map[string]json.RawMessage `json:"updatedInput,omitempty"`
	Message      string                     `json:"message,omitempty"`
}

func ReviewFeedback(res Result) string {
	label := ""
	if res.PR == nil && res.Diff != "" {
		label = DescribeDiff(res.Diff, res.Base)
	}
	return CodeFeedback(res.Annotations, label, res.PR)
}

func ReviewMessage(res Result) string {
	switch res.Decision {
	case Dismissed:
		return ReviewDismissedMessage
	case Approved:
		if len(res.Annotations) == 0 {
			return ReviewApprovedPrompt
		}
		return fill(reviewApprovedWithNotesPrompt, "feedback", strings.TrimSpace(ReviewFeedback(res)))
	}
	return ReviewFeedback(res) + "\n" + ReviewDeniedSuffix
}

func ReviewOutput(res Result, asJSON bool) (string, error) {
	msg := ReviewMessage(res)
	if !asJSON {
		return msg + "\n", nil
	}
	return jsonLine(reviewJSON{Decision: res.Decision, Message: msg})
}

type AnnotateFlags struct {
	JSON bool
	Hook bool
}

func AnnotateFeedback(file string, res Result) string {
	return fill(annotateFilePrompt, "fileHeader", annotateFileHeader, "filePath", file, "feedback", DocFeedback(res.Annotations, DocFile))
}

func AnnotateOutput(file string, res Result, f AnnotateFlags) (string, error) {
	notes := ""
	if res.Decision == Approved && len(res.Annotations) > 0 {
		notes = DocFeedback(res.Annotations, DocFile)
	}
	switch {
	case f.Hook:
		if res.Decision != Annotated {
			return "", nil
		}
		return jsonLine(hookBlock{Decision: "block", Reason: AnnotateFeedback(file, res)})
	case f.JSON:
		out := annotateJSON{Decision: res.Decision, Target: file}
		count := len(res.Annotations)
		switch res.Decision {
		case Approved:
			out.AnnotationCount = &count
			if notes != "" {
				out.Feedback = &notes
			}
		case Annotated:
			feedback := AnnotateFeedback(file, res)
			out.Feedback, out.AnnotationCount = &feedback, &count
		}
		return jsonLine(out)
	}
	switch res.Decision {
	case Dismissed:
		return "", nil
	case Approved:
		if notes == "" {
			return AnnotateApprovedMessage + "\n", nil
		}
		return fill(annotateApprovedWithNotes, "contextBlock", annotateFileHeader+": "+file+"\n\n", "feedback", notes) + "\n", nil
	}
	return AnnotateFeedback(file, res) + "\n", nil
}

func PlanHookOutput(req Request, toolInput json.RawMessage, res Result) (string, error) {
	switch res.Decision {
	case Approved:
		input := map[string]json.RawMessage{}
		if len(toolInput) > 0 {
			if err := json.Unmarshal(toolInput, &input); err != nil {
				return "", err
			}
		}
		plan, err := json.Marshal(req.Text)
		if err != nil {
			return "", err
		}
		input[planKey] = plan
		return jsonLine(permissionOutput{permissionDecision{HookEventName: "PermissionRequest", Decision: permission{Behavior: "allow", UpdatedInput: input}}})
	case Annotated:
		feedback := planDeniedFallback
		if len(res.Annotations) > 0 {
			feedback = DocFeedback(res.Annotations, DocPlan)
		}
		msg := fill(planDeniedPrompt, "toolName", PlanToolName, "planFileRule", planFileRule(PlanToolName, req.File), "feedback", feedback)
		return jsonLine(permissionOutput{permissionDecision{HookEventName: "PermissionRequest", Decision: permission{Behavior: "deny", Message: msg}}})
	}
	return "", nil
}

func jsonLine(v any) (string, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return b.String(), nil
}
