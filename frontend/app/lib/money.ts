// 金額（円・整数）を表示用にフォーマットする。例: 1200 → "¥1,200"
export function formatYen(amount: number): string {
  return `¥${amount.toLocaleString("ja-JP")}`;
}
