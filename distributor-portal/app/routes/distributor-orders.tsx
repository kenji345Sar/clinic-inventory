import { Link } from "react-router";
import type { Route } from "./+types/distributor-orders";
import { api } from "../lib/api.server";
import { formatYen } from "../lib/money";

export function meta() {
  return [{ title: "受注一覧 | 卸ポータル" }];
}

// 受注一覧画面。クリニック側で発注が確定すると、この一覧に即座に反映される。
// 現状の発注フローは「作成と同時に確定」の1ステップなので、ここに出ている発注は
// すべて確定済み(=卸が受注したとみなせる)。
export async function loader({ params }: Route.LoaderArgs) {
  const [distributors, orders] = await Promise.all([
    api.listDistributors(),
    api.listOrders(params.distributorId),
  ]);
  const distributorName =
    distributors.find((d) => d.id === params.distributorId)?.name ?? "不明な卸業者";
  // 新しい発注ほど上に出るようにする。
  const sorted = [...orders].reverse();
  return { distributorId: params.distributorId, distributorName, orders: sorted };
}

export default function DistributorOrders({ loaderData }: Route.ComponentProps) {
  const { distributorId, distributorName, orders } = loaderData;

  return (
    <main className="mx-auto max-w-5xl p-8">
      <nav className="mb-4 text-sm">
        <Link to="/" className="text-blue-600 hover:underline">
          卸業者選択
        </Link>
        <span className="mx-2 text-gray-400">/</span>
        <span>受注一覧</span>
      </nav>
      <div className="mb-6 flex items-center justify-between">
        <div>
          <p className="text-sm text-gray-500">{distributorName}</p>
          <h1 className="text-2xl font-bold">受注一覧</h1>
        </div>
        <Link
          to={`/distributors/${distributorId}/products`}
          className="text-sm text-blue-600 hover:underline"
        >
          商品マスタへ
        </Link>
      </div>

      {orders.length === 0 ? (
        <p className="rounded border border-dashed p-4 text-sm text-gray-500">
          まだ受注がありません。クリニック側で発注が確定すると、ここに表示されます。
        </p>
      ) : (
        <ul className="space-y-4">
          {orders.map((o) => (
            <li key={o.id} className="rounded border p-4">
              <div className="mb-2 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <span className="font-semibold">{o.facilityName}</span>
                  <span className="font-mono text-xs text-gray-500">{o.id}</span>
                </div>
                <span className="rounded bg-green-100 px-2 py-1 text-xs text-green-700">
                  受注済み
                </span>
              </div>
              <table className="w-full border-collapse text-sm">
                <thead>
                  <tr className="border-b bg-gray-50 text-left">
                    <th className="p-2">卸商品コード</th>
                    <th className="p-2">商品名</th>
                    <th className="p-2 text-right">数量</th>
                    <th className="p-2 text-right">単価</th>
                    <th className="p-2 text-right">金額</th>
                  </tr>
                </thead>
                <tbody>
                  {o.lines.map((l) => (
                    <tr key={l.clinicProductId} className="border-b">
                      <td className="p-2 font-mono">
                        {l.distributorProductCode || "—"}
                      </td>
                      <td className="p-2">
                        {l.distributorProductName || l.clinicProductName}
                      </td>
                      <td className="p-2 text-right">{l.quantity}</td>
                      <td className="p-2 text-right">{formatYen(l.unitPrice)}</td>
                      <td className="p-2 text-right">{formatYen(l.amount)}</td>
                    </tr>
                  ))}
                </tbody>
                <tfoot>
                  <tr className="border-t font-semibold">
                    <td className="p-2" colSpan={4}>
                      合計
                    </td>
                    <td className="p-2 text-right">{formatYen(o.totalAmount)}</td>
                  </tr>
                </tfoot>
              </table>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
