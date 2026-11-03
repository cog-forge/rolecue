package domain

// RoleProfileConfirmed is the only role profile status a session may start from.
const RoleProfileConfirmed = "confirmed"

// applicationInterviewEligible is the Application status a recruitment start needs. It mirrors the
// application feature's status value.
const applicationInterviewEligible = "interview_eligible"

// ClassifyTurn decides an answer against the turns already stored. A position already answered, as a turn
// of the same kind, with the same text is a retry and reports unchanged, so nothing is written; anything
// else at a stored position is a conflict. Otherwise the position must be the next one in the
// conversation. The caller holds the session lock, so the turns it passes are the ones the answer lands
// after.
func ClassifyTurn(turns []Turn, kind TurnKind, position int32, answer string) (unchanged bool, err error) {
	for _, turn := range turns {
		if turn.Position == position {
			if turn.Kind == kind && turn.Answer == answer {
				return true, nil
			}
			return false, ErrTurnConflict
		}
	}
	if int(position) != len(turns)+1 {
		return false, ErrTurnConflict
	}
	return false, nil
}

// AcceptAnswer applies the turn rules to an answer to a core question: ClassifyTurn, then the next core
// question must be open at that position. It returns the slot the answer is stored for. An answer after
// the last core question is a conflict.
func AcceptAnswer(turns []Turn, questionCount, position int32, answer string) (Slot, bool, error) {
	unchanged, err := ClassifyTurn(turns, TurnCore, position, answer)
	if err != nil || unchanged {
		return Slot{}, unchanged, err
	}
	slot, ok := NextCore(turns, questionCount)
	if !ok || slot.Position != position {
		return Slot{}, false, ErrTurnConflict
	}
	return slot, false, nil
}

// AcceptFollowUp applies the turn rules to an answer to a follow-up the live interviewer asked:
// ClassifyTurn, then a follow-up slot must be open at that position within followUpLimit. It returns the
// slot the answer is stored for.
func AcceptFollowUp(turns []Turn, followUpLimit, position int32, answer string) (Slot, bool, error) {
	unchanged, err := ClassifyTurn(turns, TurnFollowUp, position, answer)
	if err != nil || unchanged {
		return Slot{}, unchanged, err
	}
	slot, ok := FollowUpSlot(turns, followUpLimit)
	if !ok || slot.Position != position {
		return Slot{}, false, ErrTurnConflict
	}
	return slot, false, nil
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
