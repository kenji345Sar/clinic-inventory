import { Form, Link } from "react-router";
import type { Route } from "./+types/facility-orders";
import { ApiError, api } from "../lib/api.server";
import type { ClinicProduct } from "../lib/api.server";
import { requireAuth } from "../lib/auth.server";

export function meta() {
  return [{ title: "発注 | クリニック在庫管理" }];
}

// 発注画面。登録済みクリニック商品を卸業者ごとにまとめて表示し、
// 卸単位で数量を入れて発注する。「1発注 = 1卸」なので発注フォームも卸ごとに分ける。
export async function loader({ params, request }: Route.LoaderArgs) {
  const { accessToken } = await requireAuth(request);
  const [facilities, clinicProducts, orders] = await Promise.all([
    api.listFacilities(accessToken),
    api.listClinicProducts(accessToken, params.facilityId),
    api.listPurchaseOrders(accessToken, params.facilityId),
  ]);
  const facility = facilities.find((f) => f.id === params.facilityId);
  if (!facility) {
    throw new Response("クリニックが見つかりません", { status: 404 });
  }

  // クリニック商品を卸業者ごとにグループ化する。
  const groupsMap = new Map<
    string,
    { distributorId: string; distributorName: string; products: ClinicProduct[] }
  >();
  for (const p of clinicProducts) {
    const key = p.distributorId || "unknown";
    const group = groupsMap.get(key);
    if (group) {
      group.products.push(p);
    } else {
      groupsMap.set(key, {
        distributorId: p.distributorId,
        distributorName: p.distributorName || "（不明な卸）",
        products: [p],
      });
    }
  }
  const groups = Array.from(groupsMap.values());

  return { facility, groups, orders, clinicProducts };
}

export async function action({ params, request }: Route.ActionArgs) {
  const { accessToken } = await requireAuth(request);
  const form = await request.formData();
  const distributorId = String(form.get("distributorId") ?? "");

  // qty_<クリニック商品ID> の入力を集め、数量1以上のものだけを明細にする。
  const lines: { clinicProductId: string; quantity: number }[] = [];
  for (const [key, value] of form.entries()) {
    if (!key.startsWith("qty_")) continue;
    const quantity = Number(value);
    if (Number.isFinite(quantity) && quantity > 0) {
      lines.push({ clinicProductId: key.slice("qty_".length), quantity });
    }
  }

  if (lines.length === 0) {
    return {
      ok: false as const,
      error: "数量を1以上で入力してください。",
      distributorId,
    };
  }

  try {
    const order = await api.createPurchaseOrder(accessToken, params.facilityId, {
      distributorId,
      lines,
    });
    return { ok: true as const, error: null, distributorId, orderId: order.id };
  } catch (e) {
    if (e instanceof ApiError) {
      return { ok: false as const, error: e.message, distributorId };
    }
    throw e;
  }
}

const statusLabel: Record<string, string> = {
  draft: "下書き",
  confirmed: "確定",
};

export default function FacilityOrders({
  loaderData,
  actionData,
}: Route.ComponentProps) {
  const { facility, groups, orders, clinicProducts } = loaderData;

  // 発注履歴で明細のクリニック商品名を引くための対応表。
  const productById = new Map(clinicProducts.map((p) => [p.id, p]));

  return (
    <main className="mx-auto max-w-5xl p-8">
      <nav className="mb-4 text-sm">
        <Link to="/" className="text-blue-600 hover:underline">
          クリニック選択
        </Link>
        <span className="mx-2 text-gray-400">/</span>
        <span>{facility.name}</span>
        <span className="mx-2 text-gray-400">/</span>
        <span>発注</span>
      </nav>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold">発注 — {facility.name}</h1>
        <Link
          to={`/facilities/${facility.id}/products`}
          className="text-sm text-blue-600 hover:underline"
        >
          商品マスタへ
        </Link>
      </div>

      <section className="mb-10">
        <h2 className="mb-3 text-lg font-semibold">発注する商品と数量を入力</h2>
        {groups.length === 0 ? (
          <p className="rounded border border-dashed p-4 text-sm text-gray-500">
            発注できる商品がありません。先に
            <Link
              to={`/facilities/${facility.id}/products`}
              className="mx-1 text-blue-600 hover:underline"
            >
              商品マスタ
            </Link>
            でクリニック商品を登録してください。
          </p>
        ) : (
          <div className="space-y-6">
            {groups.map((group) => {
              const message =
                actionData && actionData.distributorId === group.distributorId
                  ? actionData
                  : null;
              return (
                <Form
                  key={group.distributorId || "unknown"}
                  method="post"
                  className="rounded border p-4"
                >
                  <input
                    type="hidden"
                    name="distributorId"
                    value={group.distributorId}
                  />
                  <h3 className="mb-3 font-semibold">
                    卸業者: {group.distributorName}
                  </h3>
                  <table className="w-full border-collapse text-sm">
                    <thead>
                      <tr className="border-b bg-gray-50 text-left">
                        <th className="p-2">商品コード</th>
                        <th className="p-2">商品名</th>
                        <th className="p-2 text-right">発注点</th>
                        <th className="p-2 text-right">発注数量</th>
                      </tr>
                    </thead>
                    <tbody>
                      {group.products.map((p) => (
                        <tr key={p.id} className="border-b">
                          <td className="p-2 font-mono">{p.productCode}</td>
                          <td className="p-2">{p.name}</td>
                          <td className="p-2 text-right text-gray-500">
                            {p.reorderPoint}
                          </td>
                          <td className="p-2 text-right">
                            <input
                              name={`qty_${p.id}`}
                              type="number"
                              min={0}
                              defaultValue={0}
                              className="w-24 rounded border p-1 text-right"
                            />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  {message && !message.ok && (
                    <p className="mt-3 text-sm text-red-600">{message.error}</p>
                  )}
                  {message?.ok && (
                    <p className="mt-3 text-sm text-green-700">
                      発注を確定しました。
                    </p>
                  )}
                  <button
                    type="submit"
                    className="mt-4 rounded bg-blue-600 px-4 py-2 text-white hover:bg-blue-700"
                  >
                    この卸に発注
                  </button>
                </Form>
              );
            })}
          </div>
        )}
      </section>

      <section>
        <h2 className="mb-3 text-lg font-semibold">発注履歴</h2>
        {orders.length === 0 ? (
          <p className="rounded border border-dashed p-4 text-sm text-gray-500">
            まだ発注がありません。
          </p>
        ) : (
          <ul className="space-y-3">
            {orders.map((o) => (
              <li key={o.id} className="rounded border p-4">
                <div className="mb-2 flex items-center justify-between">
                  <span className="font-mono text-xs text-gray-500">{o.id}</span>
                  <span className="rounded bg-green-100 px-2 py-1 text-xs text-green-700">
                    {statusLabel[o.status] ?? o.status}
                  </span>
                </div>
                <ul className="text-sm">
                  {o.lines.map((l) => {
                    const product = productById.get(l.clinicProductId);
                    return (
                      <li
                        key={l.clinicProductId}
                        className="flex justify-between border-t py-1"
                      >
                        <span>
                          {product
                            ? `${product.productCode} ${product.name}`
                            : l.clinicProductId}
                        </span>
                        <span className="text-gray-600">{l.quantity} 個</span>
                      </li>
                    );
                  })}
                </ul>
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  );
}
