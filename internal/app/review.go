package app

import (
	"log"

	"github.com/ZerGo0/editor/internal/review"
	"github.com/ZerGo0/editor/internal/reviewui"
	"github.com/ZerGo0/editor/internal/workspace"
	"github.com/egoist/mygo"
)

func (a *App) openReview(path string) bool {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Printf("review request: %v", err)
		return false
	}
	s, req, err := review.OpenRequest(dir, path)
	if err != nil {
		log.Printf("review request: %v", err)
		return false
	}
	root, err := canonical(req.Folder)
	if err != nil {
		log.Printf("review request: %v", err)
		return false
	}
	pw := a.openProject(root)
	pw.ws.Update(func() { pw.ws.OpenReview(s, req) })
	return true
}

func (a *App) registerReviewCommands() {
	ws, add := a.inWorkspace, a.addCommand
	review := func(id string) func(*mygo.Window) {
		return ws(func(w *workspace.Workspace) { w.RunReview(id) })
	}
	add(reviewui.DecideID, categoryReview, "Approve or Send Feedback", review(reviewui.DecideID))
	add(reviewui.ApproveWithNotesID, categoryReview, "Approve with Notes", review(reviewui.ApproveWithNotesID))
	add(reviewui.AddGlobalCommentID, categoryReview, "Add Global Comment", review(reviewui.AddGlobalCommentID))
	add(reviewui.CloseID, categoryReview, "Close Review", review(reviewui.CloseID))
}
