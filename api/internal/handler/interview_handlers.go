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
}

// NewInterviewHandlers bundles the interview handlers.
func NewInterviewHandlers(
	sessions *InterviewHandler,
	history *InterviewHistoryHandler,
	report *InterviewReportHandler,
	recording *InterviewRecordingHandler,
	live *InterviewLiveHandler,
) *InterviewHandlers {
	return &InterviewHandlers{Sessions: sessions, History: history, Report: report, Recording: recording, Live: live}
}
