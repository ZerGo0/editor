package issueui

const actionCategory = "GitHub Issues"

type Action struct {
	ID       string
	Title    string
	Category string
	Key      string
}

const (
	ActionRefresh             = "issues.refresh"
	ActionCreate              = "issues.create"
	ActionCreateFromSelection = "issues.createFromSelection"
	ActionStartWorking        = "issues.startWorking"
	ActionStopWorking         = "issues.stopWorking"
	ActionContinueWorking     = "issues.continueWorking"
	ActionCopyPermalink       = "issues.copyPermalink"
	ActionOpenPermalink       = "issues.openPermalink"
)

var Actions = []Action{
	{ID: ActionRefresh, Title: "Refresh Issues", Category: actionCategory},
	{ID: ActionCreate, Title: "Create Issue", Category: actionCategory},
	{ID: ActionCreateFromSelection, Title: "Create Issue from Selection", Category: actionCategory},
	{ID: ActionStartWorking, Title: "Start Working on Issue", Category: actionCategory},
	{ID: ActionStopWorking, Title: "Stop Working on Issue", Category: actionCategory},
	{ID: ActionContinueWorking, Title: "Continue Working on Issue", Category: actionCategory},
	{ID: ActionCopyPermalink, Title: "Copy GitHub Permalink", Category: actionCategory},
	{ID: ActionOpenPermalink, Title: "Open GitHub Permalink", Category: actionCategory},
}

func (i *Issues) Run(id string) bool {
	switch id {
	case ActionRefresh:
		i.refresh()
	case ActionCreate:
		i.openCreate(createDraft{})
	case ActionCreateFromSelection:
		i.createFromActiveSelection()
	case ActionStartWorking:
		i.startWorkingAction()
	case ActionStopWorking:
		i.stopWorking()
	case ActionContinueWorking:
		i.continueWorkingAction()
	case ActionCopyPermalink:
		i.withActiveSelection(i.CopyPermalink)
	case ActionOpenPermalink:
		i.withActiveSelection(i.OpenPermalink)
	default:
		return false
	}
	return true
}
