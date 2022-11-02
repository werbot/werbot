type Event = "connextSuccess" | "connextError" | "connextWarning" | "connextInfo";

export function showMessage(message: string, event: Event = "connextSuccess"): any {
  const eventMessage = new CustomEvent(event, {
    detail: message,
  });
  dispatchEvent(eventMessage);
}
