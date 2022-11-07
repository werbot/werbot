type Method = "full" | "lite";

export function toDate(timestamp: any, format: Method = "full"): string {
  const dateString: string = "";
  let dateMath: number = timestamp.seconds * 1000;
  if (timestamp.nanos) {
    dateMath = dateMath + timestamp.nanos / 1e6;
  }

  const date = new Date(dateMath);
  if (format === "lite") {
    return date.toLocaleDateString();
  }

  return date.toLocaleString();
}
