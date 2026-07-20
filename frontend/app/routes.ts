import { type RouteConfig, index, route } from "@react-router/dev/routes";

export default [
  index("routes/home.tsx"),
  route("facilities/:facilityId/products", "routes/facility-products.tsx"),
] satisfies RouteConfig;
