// Goバックエンド（cmd/api）へのサーバーサイドAPIクライアント。
// loader/actionからのみ使う（ブラウザから直接呼ばない構成なのでCORS不要）。

const API_BASE = process.env.API_BASE ?? "http://localhost:8080";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(
  path: string,
  accessToken: string,
  init?: RequestInit,
): Promise<T> {
  const authHeader: Record<string, string> = accessToken
    ? { Authorization: `Bearer ${accessToken}` }
    : {};
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...authHeader,
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

export interface Facility {
  id: string;
  name: string;
  facilityType: "medical" | "dental" | "vet";
  corporationId: string;
  groupId: string | null;
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

export interface ClinicProduct {
  id: string;
  facilityId: string;
  productCode: string;
  name: string;
  distributorProductId: string;
  distributorId: string;
  distributorName: string;
  janCode: string;
  unitPrice: number;
  reorderPoint: number;
}

export interface PurchaseOrderLine {
  clinicProductId: string;
  quantity: number;
  unitPrice: number;
  amount: number;
}

export interface PurchaseOrder {
  id: string;
  facilityId: string;
  distributorId: string;
  status: "draft" | "confirmed";
  lines: PurchaseOrderLine[];
  totalAmount: number;
}

// 各メソッドは第1引数に accessToken を取り、backend への Bearer 認証に使う。
// トークンは loader/action が requireAuth() で取得して渡す（AUTH_DISABLED 時は空文字）。
export const api = {
  listFacilities: (accessToken: string) =>
    request<Facility[]>("/api/facilities", accessToken),
  listDistributors: (accessToken: string) =>
    request<Distributor[]>("/api/distributors", accessToken),
  listDistributorProducts: (accessToken: string, distributorId: string) =>
    request<DistributorProduct[]>(
      `/api/distributors/${distributorId}/products`,
      accessToken,
    ),
  listClinicProducts: (accessToken: string, facilityId: string) =>
    request<ClinicProduct[]>(
      `/api/facilities/${facilityId}/products`,
      accessToken,
    ),
  registerClinicProduct: (
    accessToken: string,
    facilityId: string,
    input: {
      productCode: string;
      name?: string;
      distributorProductId: string;
      janCode?: string;
      unitPrice: number;
      reorderPoint: number;
    },
  ) =>
    request<ClinicProduct>(`/api/facilities/${facilityId}/products`, accessToken, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  listPurchaseOrders: (accessToken: string, facilityId: string) =>
    request<PurchaseOrder[]>(`/api/facilities/${facilityId}/orders`, accessToken),
  createPurchaseOrder: (
    accessToken: string,
    facilityId: string,
    input: {
      distributorId: string;
      lines: { clinicProductId: string; quantity: number }[];
    },
  ) =>
    request<PurchaseOrder>(`/api/facilities/${facilityId}/orders`, accessToken, {
      method: "POST",
      body: JSON.stringify(input),
    }),
};
