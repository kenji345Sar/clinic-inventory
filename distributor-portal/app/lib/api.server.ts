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
  /** 標準単価(税抜・円)。nullは全医院共通の定価が無いことを表す(医院別単価の卸・単価未提供の卸) */
  unitPrice: number | null;
  /** 医院別単価が設定されている医院数。0なら医院別単価は無い */
  facilityPriceCount: number;
  discontinued: boolean;
}

export interface FacilityPrice {
  facilityId: string;
  facilityName: string;
  unitPrice: number;
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
      /** nullは単価を公表しないことを表す */
      unitPrice: number | null;
    },
  ) =>
    request<DistributorProduct>(
      `/api/portal/distributors/${distributorId}/products`,
      { method: "POST", body: JSON.stringify(input) },
    ),
  // 1商品の医院別単価の内訳。一覧では件数だけ返るため、選択時にこれを取る。
  listFacilityPrices: (distributorId: string, productId: string) =>
    request<FacilityPrice[]>(
      `/api/portal/distributors/${distributorId}/products/${productId}/facility-prices`,
    ),
  listOrders: (distributorId: string) =>
    request<Order[]>(`/api/portal/distributors/${distributorId}/orders`),
};
