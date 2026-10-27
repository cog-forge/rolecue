export const roleProfileKeys = {
  all: (userId: string) => ["role-profiles", userId] as const,
  lists: (userId: string) => [...roleProfileKeys.all(userId), "list"] as const,
  list: (userId: string, offset: number) =>
    [...roleProfileKeys.lists(userId), offset] as const,
  detail: (userId: string, id: string) =>
    [...roleProfileKeys.all(userId), "detail", id] as const,
};
