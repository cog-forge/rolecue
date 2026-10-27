package handler

// InterviewHandlers groups the interview HTTP handlers so the router takes one dependency for the
// whole interview surface: sessions, history, the evaluation report and recruitment recordings.
type InterviewHandlers struct {
	Sessions  *InterviewHandler
	History   *InterviewHistoryHandler
	Report    *InterviewReportHandler
	Recording *InterviewRecordingHandler
}

// NewInterviewHandlers bundles the interview handlers.
func NewInterviewHandlers(
	sessions *InterviewHandler,
	history *InterviewHistoryHandler,
	report *InterviewReportHandler,
	recording *InterviewRecordingHandler,
) *InterviewHandlers {
	return &InterviewHandlers{Sessions: sessions, History: history, Report: report, Recording: recording}
}
