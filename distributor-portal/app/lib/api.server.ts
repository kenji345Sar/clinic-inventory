// Goバックエンド（cmd/api）の卸ポータル向けエンドポイント(/api/portal/...)への
// サーバーサイドAPIクライアント。loader/actionからのみ使う。
//
// 卸業者はまだAuth0アカウントを持たない(docs/requirements.md 8章「後続」)ため、
// このアプリは未認証で動く。トークンは送らない。

const API_BASE = process.env.API_BASE ?? "http://localhost:8080";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as {
      error?: string;
    } | null;
    throw new ApiError(res.status, body?.error ?? `APIエラー (${res.status})`);
  }
  return (await res.json()) as T;
}

export interface Distributor {
  id: string;
  name: string;
}

export interface DistributorProduct {
  id: string;
  distributorId: string;
  distributorProductCode: string;
  name: string;
  vendorName: string;
  vendorProductCode: string;
  janCode: string;
  unitPrice: number;
  discontinued: boolean;
}

export interface OrderLine {
  clinicProductId: string;
  clinicProductCode: string;
  clinicProductName: string;
  distributorProductId: string;
  distributorProductCode: string;
  distributorProductName: string;
  quantity: number;
  unitPrice: number;
  amount: number;
}

export interface Order {
  id: string;
  facilityId: string;
  facilityName: string;
  distributorId: string;
  status: "draft" | "confirmed";
  lines: OrderLine[];
  totalAmount: number;
}

export const api = {
  listDistributors: () => request<Distributor[]>("/api/portal/distributors"),
  listProducts: (distributorId: string) =>
    request<DistributorProduct[]>(
      `/api/portal/distributors/${distributorId}/products`,
    ),
  registerProduct: (
    distributorId: string,
    input: {
      distributorProductCode: string;
      name: string;
      vendorName: string;
      vendorProductCode?: string;
      janCode?: string;
      unitPrice: number;
    },
  ) =>
    request<DistributorProduct>(
      `/api/portal/distributors/${distributorId}/products`,
      { method: "POST", body: JSON.stringify(input) },
    ),
  listOrders: (distributorId: string) =>
    request<Order[]>(`/api/portal/distributors/${distributorId}/orders`),
};
