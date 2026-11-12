import { apiClient } from "@/lib/api/client";
import {
  bankEnvelopeSchema,
  generatedEnvelopeSchema,
  questionEnvelopeSchema,
} from "../schemas/question-bank-schema";

export const questionBankKeys = {
  all: (userId: string) => ["recruiter-question-bank", userId] as const,
  detail: (userId: string, postingId: string) =>
    [...questionBankKeys.all(userId), postingId] as const,
};

const bankPath = (postingId: string) =>
  `/job-postings/${encodeURIComponent(postingId)}/questions`;
const questionPath = (postingId: string, questionId: string) =>
  `${bankPath(postingId)}/${encodeURIComponent(questionId)}`;

export async function getBank(postingId: string, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(bankPath(postingId), {
    signal,
  });
  return bankEnvelopeSchema.parse(response.data).data;
}

export async function generateBank(postingId: string) {
  const response = await apiClient.post<unknown>(
    `${bankPath(postingId)}/generate`,
  );
  return generatedEnvelopeSchema.parse(response.data).data.questions;
}

export async function addQuestion(postingId: string, content: string) {
  const response = await apiClient.post<unknown>(bankPath(postingId), {
    content,
  });
  return questionEnvelopeSchema.parse(response.data).data;
}

export async function updateQuestion(
  postingId: string,
  questionId: string,
  content: string,
  previousRevision?: string,
) {
  const response = await apiClient.patch<unknown>(
    questionPath(postingId, questionId),
    {
      content,
      ...(previousRevision ? { previous_revision: previousRevision } : {}),
    },
  );
  return questionEnvelopeSchema.parse(response.data).data;
}

export async function deleteQuestion(postingId: string, questionId: string) {
  await apiClient.delete(questionPath(postingId, questionId));
}

export type GradingPatch = {
  rubric_id: string | null;
  expected_answer: string | null;
  previous_revision: string;
};
export async function saveGrading(
  postingId: string,
  questionId: string,
  body: GradingPatch,
) {
  const response = await apiClient.patch<unknown>(
    questionPath(postingId, questionId),
    body,
  );
  return questionEnvelopeSchema.parse(response.data).data;
}
export async function applyRubric(
  postingId: string,
  body: {
    rubric_id: string;
    question_ids: string[];
    previous_revisions: Record<string, string>;
  },
) {
  const response = await apiClient.post<unknown>(
    `${bankPath(postingId)}/apply-rubric`,
    body,
  );
  return generatedEnvelopeSchema.parse(response.data).data.questions;
}
