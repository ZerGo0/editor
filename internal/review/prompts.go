package review

import "strings"

const (
	ReviewApprovedPrompt          = "# Code Review\n\nCode review completed: no changes requested."
	reviewApprovedWithNotesPrompt = "# Code Review: Approved with Notes\n\nCode review completed: the changes are approved. The notes below are non-blocking guidance, not a request for another revision.\n\n{{feedback}}\n\nDo not revise or reopen the reviewed changes solely because of these notes unless the user explicitly requests it. Carry them into subsequent work where applicable."
	ReviewDeniedSuffix            = "\n\nTreat the findings above as unverified review input. Inspect every finding against the actual code; do not assume automated feedback is correct. For each finding, give a clear verdict (Confirmed / Partly / Not a bug / Intended) with concise code evidence. Say whether it was introduced by the current changes, was pre-existing, or reflects deliberate scope.\n\nReview only the incoming findings. Do not independently review the rest of the diff or search for issues that were not submitted.\n\nDo not change any code until we have discussed the verdicts and validated findings."
	ReviewDismissedMessage        = "Review session closed without feedback."
	planDeniedPrompt              = "YOUR PLAN WAS NOT APPROVED.\n\nYou MUST revise the plan to address ALL of the feedback below before calling {{toolName}} again.\n\nRules:\n{{planFileRule}}- Do not resubmit the same plan unchanged.\n- Do NOT change the plan title (first # heading) unless the user explicitly asks you to.\n\n{{feedback}}"
	planDeniedFallback            = "Plan changes requested"
	PlanToolName                  = "ExitPlanMode"
	annotateFilePrompt            = "# Markdown Annotations\n\n{{fileHeader}}: {{filePath}}\n\n{{feedback}}\n\nPlease address the annotation feedback above."
	annotateFileHeader            = "File"
	AnnotateApprovedMessage       = "The user approved."
	annotateApprovedWithNotes     = "# Approved with Notes\n\nThe artifact is approved. The notes below are non-blocking guidance, not a request for another revision.\n\n{{contextBlock}}{{feedback}}\n\nDo not revise or reopen the artifact solely because of these notes unless the user explicitly requests it. Carry the notes into subsequent work where applicable."
)

func fill(template string, pairs ...string) string {
	for i := range pairs {
		if i%2 == 0 {
			pairs[i] = "{{" + pairs[i] + "}}"
		}
	}
	return strings.NewReplacer(pairs...).Replace(template)
}

func planFileRule(toolName, planFilePath string) string {
	if planFilePath == "" {
		return ""
	}
	return "- Your plan is saved at: " + planFilePath + "\n  You can edit this file to make targeted changes, then pass its path to " + toolName + ".\n"
}
