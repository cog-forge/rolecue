package domain

// RoleProfileConfirmed is the only role profile status a session may start from.
const RoleProfileConfirmed = "confirmed"

// applicationInterviewEligible is the Application status a recruitment start needs. It mirrors the
// application feature's status value.
const applicationInterviewEligible = "interview_eligible"

// ClassifyTurn decides an answer against the turns already stored. A position already answered with the
// same text is a retry and reports unchanged, so nothing is written; with other text it is a conflict.
// Otherwise the position must be the next unanswered question and must exist in the bank. The caller
// holds the session lock, so the turns it passes are the ones the answer lands after.
func ClassifyTurn(turns []Turn, questionCount int, position int32, answer string) (unchanged bool, err error) {
	for _, turn := range turns {
		if turn.Position == position {
			if turn.Answer == answer {
				return true, nil
			}
			return false, ErrTurnConflict
		}
	}
	if int(position) != len(turns)+1 || int(position) > questionCount {
		return false, ErrTurnConflict
	}
	return false, nil
}

// CheckRecruitmentStart is the Application and capacity rule of a recruitment start, in the order the
// Candidate sees the failures. slot is the posting's funded interview_slot and consumed the interviews
// that already used one.
func CheckRecruitmentStart(status string, withinDeadline bool, slot *int32, consumed int64) error {
	if status != applicationInterviewEligible || !withinDeadline {
		return ErrApplicationNotEligible
	}
	if slot == nil {
		return ErrCapacityNotFunded
	}
	if consumed >= int64(*slot) {
		return ErrCapacityFull
	}
	return nil
}
