import { api } from "./client";
import type { NotificationList } from "@/types";

export const notificationsApi = {
  list: (signal?: AbortSignal) => api<NotificationList>("/notifications", { signal }),
  read: (id: string) => api<{ ok: boolean }>(`/notifications/${id}/read`, { method: "POST" }),
  readAll: () => api<{ ok: boolean }>("/notifications/read-all", { method: "POST" }),
};
