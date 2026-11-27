package llmerr

import "github.com/cog-forge/rolecue/api/pkg/apperror"

// aiBusyMessage is the only text a busy failure carries; the provider's own words never reach the caller.
const aiBusyMessage = "AI service is busy"

// Wrap turns the failure of a model generation into an API error. When the cause is provider congestion (a
// timeout, a 429, a 5xx or a network error, see Transient) it is AI_SERVICE_BUSY, which tells the user to try
// again in a few minutes; any other cause keeps the code and message of the caller. The cause stays attached.
func Wrap(code apperror.Code, message string, cause error) error {
	if Transient(cause) {
		return apperror.Wrap(apperror.CodeAIServiceBusy, aiBusyMessage, cause)
	}
	return apperror.Wrap(code, message, cause)
}
