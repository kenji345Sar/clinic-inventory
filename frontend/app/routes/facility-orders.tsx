import { useState } from "react";
import { Form, Link } from "react-router";
import type { Route } from "./+types/facility-orders";
import { ApiError, api } from "../lib/api.server";
import type { ClinicProduct, PurchaseOrder } from "../lib/api.server";
import { requireAuth } from "../lib/auth.server";
import { formatYen } from "../lib/money";

export function meta() {
  return [{ title: "発注 | クリニック在庫管理" }];
}

// 発注画面。登録済みクリニック商品を卸業者ごとにまとめて表示し、
// 卸単位で数量を入れてカートに積む。「1発注 = 1卸」なので発注フォームも卸ごとに分ける。
// カートの中身（下書き）は「発注する」で確定するまでは発注履歴には出ない。
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

  const draftOrders = orders.filter((o) => o.status === "draft");
  const confirmedOrders = orders.filter((o) => o.status === "confirmed");

  return { facility, groups, draftOrders, confirmedOrders, clinicProducts };
}

export async function action({ params, request }: Route.ActionArgs) {
  const { accessToken } = await requireAuth(request);
  const form = await request.formData();
  const intent = String(form.get("intent") ?? "addToCart");

  // カートに積んだ発注の確定・取消。
  if (intent === "confirm" || intent === "remove") {
    const orderId = String(form.get("orderId") ?? "");
    try {
      if (intent === "confirm") {
        await api.confirmPurchaseOrder(accessToken, params.facilityId, orderId);
      } else {
        await api.deletePurchaseOrder(accessToken, params.facilityId, orderId);
      }
      return { ok: true as const, error: null, intent, orderId };
    } catch (e) {
      if (e instanceof ApiError) {
        return { ok: false as const, error: e.message, intent, orderId };
      }
      throw e;
    }
  }

  // カートへの追加。qty_<クリニック商品ID> の入力を集め、数量1以上のものだけを明細にする。
  const distributorId = String(form.get("distributorId") ?? "");
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
      intent: "addToCart" as const,
      distributorId,
    };
  }

  try {
    const order = await api.saveDraftPurchaseOrder(accessToken, params.facilityId, {
      distributorId,
      lines,
    });
    return {
      ok: true as const,
      error: null,
      intent: "addToCart" as const,
      distributorId,
      orderId: order.id,
    };
  } catch (e) {
    if (e instanceof ApiError) {
      return {
        ok: false as const,
        error: e.message,
        intent: "addToCart" as const,
        distributorId,
      };
    }
    throw e;
  }
}

type ActionData = Awaited<ReturnType<typeof action>>;

interface OrderGroup {
  distributorId: string;
  distributorName: string;
  products: ClinicProduct[];
}

// 卸業者ごとの発注フォーム。数量を入れると単価から金額・小計をライブ計算して表示する。
// 送信するのは数量のみで、単価はサーバ側でクリニック商品からスナップショットされる。
function GroupOrderForm({
  group,
  message,
}: {
  group: OrderGroup;
  message: ActionData | null;
}) {
  const [qty, setQty] = useState<Record<string, number>>({});
  const subtotal = group.products.reduce(
    (sum, p) => sum + (qty[p.id] ?? 0) * p.unitPrice,
    0,
  );

  return (
    <Form method="post" className="rounded border p-4">
      <input type="hidden" name="intent" value="addToCart" />
      <input type="hidden" name="distributorId" value={group.distributorId} />
      <h3 className="mb-3 font-semibold">卸業者: {group.distributorName}</h3>
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="border-b bg-gray-50 text-left">
            <th className="p-2">商品コード</th>
            <th className="p-2">商品名</th>
            <th className="p-2 text-right">単価</th>
            <th className="p-2 text-right">発注点</th>
            <th className="p-2 text-right">発注数量</th>
            <th className="p-2 text-right">金額</th>
          </tr>
        </thead>
        <tbody>
          {group.products.map((p) => {
            const q = qty[p.id] ?? 0;
            return (
              <tr key={p.id} className="border-b">
                <td className="p-2 font-mono">{p.productCode}</td>
                <td className="p-2">{p.name}</td>
                <td className="p-2 text-right">{formatYen(p.unitPrice)}</td>
                <td className="p-2 text-right text-gray-500">
                  {p.reorderPoint}
                </td>
                <td className="p-2 text-right">
                  <input
                    name={`qty_${p.id}`}
                    type="number"
                    min={0}
                    defaultValue={0}
                    onChange={(e) =>
                      setQty((prev) => ({
                        ...prev,
                        [p.id]: Number(e.target.value) || 0,
                      }))
                    }
                    className="w-24 rounded border p-1 text-right"
                  />
                </td>
                <td className="p-2 text-right">{formatYen(q * p.unitPrice)}</td>
              </tr>
            );
          })}
        </tbody>
        <tfoot>
          <tr className="border-t font-semibold">
            <td className="p-2" colSpan={5}>
              小計
            </td>
            <td className="p-2 text-right">{formatYen(subtotal)}</td>
          </tr>
        </tfoot>
      </table>
      {message && !message.ok && (
        <p className="mt-3 text-sm text-red-600">{message.error}</p>
      )}
      {message?.ok && (
        <p className="mt-3 text-sm text-green-700">カートに追加しました。</p>
      )}
      <button
        type="submit"
        className="mt-4 rounded bg-blue-600 px-4 py-2 text-white hover:bg-blue-700"
      >
        カートに追加
      </button>
    </Form>
  );
}

// カートに積んだ発注（下書き）1件分のカード。明細・小計に加え、確定・取消のボタンを持つ。
function CartOrderCard({
  order,
  distributorName,
  productById,
  message,
}: {
  order: PurchaseOrder;
  distributorName: string;
  productById: Map<string, ClinicProduct>;
  message: ActionData | null;
}) {
  return (
    <li className="rounded border p-4">
      <div className="mb-2 flex items-center justify-between">
        <span className="font-semibold">{distributorName}</span>
        <span className="rounded bg-yellow-100 px-2 py-1 text-xs text-yellow-800">
          下書き
        </span>
      </div>
      <ul className="text-sm">
        {order.lines.map((l) => {
          const product = productById.get(l.clinicProductId);
          return (
            <li
              key={l.clinicProductId}
              className="flex justify-between border-t py-1"
            >
              <span>
                {product ? `${product.productCode} ${product.name}` : l.clinicProductId}
              </span>
              <span className="text-gray-600">
                {formatYen(l.unitPrice)} × {l.quantity} = {formatYen(l.amount)}
              </span>
            </li>
          );
        })}
      </ul>
      <div className="mt-2 flex justify-end border-t pt-2 text-sm font-semibold">
        合計 {formatYen(order.totalAmount)}
      </div>
      {message && !message.ok && (
        <p className="mt-2 text-sm text-red-600">{message.error}</p>
      )}
      <div className="mt-3 flex justify-end gap-2">
        <Form method="post">
          <input type="hidden" name="orderId" value={order.id} />
          <button
            type="submit"
            name="intent"
            value="remove"
            className="rounded border border-gray-300 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50"
          >
            取消
          </button>
        </Form>
        <Form method="post">
          <input type="hidden" name="orderId" value={order.id} />
          <button
            type="submit"
            name="intent"
            value="confirm"
            className="rounded bg-blue-600 px-4 py-2 text-sm text-white hover:bg-blue-700"
          >
            発注する
          </button>
        </Form>
      </div>
    </li>
  );
}

export default function FacilityOrders({
  loaderData,
  actionData,
}: Route.ComponentProps) {
  const { facility, groups, draftOrders, confirmedOrders, clinicProducts } = loaderData;

  // 発注履歴・カードで明細のクリニック商品名を引くための対応表。
  const productById = new Map(clinicProducts.map((p) => [p.id, p]));
  const distributorNameById = new Map(
    groups.map((g) => [g.distributorId, g.distributorName]),
  );

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
                actionData &&
                actionData.intent === "addToCart" &&
                actionData.distributorId === group.distributorId
                  ? actionData
                  : null;
              return (
                <GroupOrderForm
                  key={group.distributorId || "unknown"}
                  group={group}
                  message={message}
                />
              );
            })}
          </div>
        )}
      </section>

      <section className="mb-10">
        <h2 className="mb-3 text-lg font-semibold">カート</h2>
        {draftOrders.length === 0 ? (
          <p className="rounded border border-dashed p-4 text-sm text-gray-500">
            カートは空です。上のフォームから商品をカートに追加してください。
          </p>
        ) : (
          <ul className="space-y-3">
            {draftOrders.map((o) => {
              const message =
                actionData &&
                (actionData.intent === "confirm" || actionData.intent === "remove") &&
                actionData.orderId === o.id
                  ? actionData
                  : null;
              return (
                <CartOrderCard
                  key={o.id}
                  order={o}
                  distributorName={distributorNameById.get(o.distributorId) ?? "（不明な卸）"}
                  productById={productById}
                  message={message}
                />
              );
            })}
          </ul>
        )}
      </section>

      <section>
        <h2 className="mb-3 text-lg font-semibold">発注履歴</h2>
        {confirmedOrders.length === 0 ? (
          <p className="rounded border border-dashed p-4 text-sm text-gray-500">
            まだ発注がありません。
          </p>
        ) : (
          <ul className="space-y-3">
            {confirmedOrders.map((o) => (
              <li key={o.id} className="rounded border p-4">
                <div className="mb-2 flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <span className="font-mono text-xs text-gray-500">{o.id}</span>
                    {o.confirmedAt && (
                      <span className="text-xs text-gray-500">
                        発注日時:{" "}
                        {new Date(o.confirmedAt).toLocaleString("ja-JP", {
                          dateStyle: "medium",
                          timeStyle: "short",
                        })}
                      </span>
                    )}
                  </div>
                  <span className="rounded bg-green-100 px-2 py-1 text-xs text-green-700">
                    確定
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
                        <span className="text-gray-600">
                          {formatYen(l.unitPrice)} × {l.quantity} ={" "}
                          {formatYen(l.amount)}
                        </span>
                      </li>
                    );
                  })}
                </ul>
                <div className="mt-2 flex justify-end border-t pt-2 text-sm font-semibold">
                  合計 {formatYen(o.totalAmount)}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  );
}
