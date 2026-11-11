package handler

// InterviewHandlers groups the interview HTTP handlers so the router takes one dependency for the
// whole interview surface: sessions, history, the evaluation report, recruitment recordings and the
// live interview socket.
type InterviewHandlers struct {
	Sessions  *InterviewHandler
	History   *InterviewHistoryHandler
	Report    *InterviewReportHandler
	Recording *InterviewRecordingHandler
	Live      *InterviewLiveHandler
	// Guidelines is the Admin side of the interview behavior (KAN-276): it shapes the interviewer, so it
	// travels with the interview handlers and is routed under /admin.
	Guidelines *AdminGuidelinesHandler
}

// NewInterviewHandlers bundles the interview handlers.
func NewInterviewHandlers(
	sessions *InterviewHandler,
	history *InterviewHistoryHandler,
	report *InterviewReportHandler,
	recording *InterviewRecordingHandler,
	live *InterviewLiveHandler,
	guidelines *AdminGuidelinesHandler,
) *InterviewHandlers {
	return &InterviewHandlers{Sessions: sessions, History: history, Report: report, Recording: recording, Live: live, Guidelines: guidelines}
}
