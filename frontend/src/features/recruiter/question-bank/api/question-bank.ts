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
) {
  const response = await apiClient.patch<unknown>(
    questionPath(postingId, questionId),
    { content },
  );
  return questionEnvelopeSchema.parse(response.data).data;
}

export async function deleteQuestion(postingId: string, questionId: string) {
  await apiClient.delete(questionPath(postingId, questionId));
}
