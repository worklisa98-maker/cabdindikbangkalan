import { clearToken, getToken, TOKEN_KEY } from "@/lib/auth";

export const IDLE_TIMEOUT_MS = 10 * 60 * 1000;
export const LAST_ACTIVITY_KEY = "cabdin_last_activity";
export const LOGOUT_BROADCAST_KEY = "cabdin_logout_broadcast";

type LogoutReason = "idle" | "broadcast";

type IdleLogoutOptions = {
  onLogout: (reason: LogoutReason) => void;
};

const activityEvents = [
  "mousemove",
  "mousedown",
  "click",
  "scroll",
  "keydown",
  "touchstart",
] as const;

export const startIdleLogout = ({ onLogout }: IdleLogoutOptions) => {
  let timer: number | null = null;

  const resetTimer = () => {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(LAST_ACTIVITY_KEY, String(Date.now()));
    if (timer) {
      window.clearTimeout(timer);
    }
    timer = window.setTimeout(() => {
      clearToken();
      window.localStorage.setItem(LOGOUT_BROADCAST_KEY, String(Date.now()));
      onLogout("idle");
    }, IDLE_TIMEOUT_MS);
  };

  const handleStorage = (event: StorageEvent) => {
    if (!event.key) return;
    if (event.key === LOGOUT_BROADCAST_KEY) {
      clearToken();
      onLogout("broadcast");
      return;
    }
    if (event.key === LAST_ACTIVITY_KEY) {
      resetTimer();
      return;
    }
    if (event.key === TOKEN_KEY && !event.newValue) {
      onLogout("broadcast");
    }
  };

  const handleActivity = () => resetTimer();

  if (getToken()) {
    resetTimer();
  }

  activityEvents.forEach((eventName) => {
    window.addEventListener(eventName, handleActivity, { passive: true });
  });
  window.addEventListener("storage", handleStorage);

  return () => {
    if (timer) {
      window.clearTimeout(timer);
    }
    activityEvents.forEach((eventName) => {
      window.removeEventListener(eventName, handleActivity);
    });
    window.removeEventListener("storage", handleStorage);
  };
};
