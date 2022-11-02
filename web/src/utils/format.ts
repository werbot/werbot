export function costFormat(cost: number): string {
  if (!cost) {
    return "0";
  }
  return (cost / 100).toFixed(2);
}

export function toInfinity(num: number): string {
  if (num == -1) {
    return "∞";
  }
  return String(num);
}

export function upperCaseWorld(world: string) {
  return world.charAt(0).toUpperCase() + world.slice(1);
}
