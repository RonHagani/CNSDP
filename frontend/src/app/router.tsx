import { type RouteObject, createBrowserRouter, Navigate } from "react-router-dom";
import { AlertInvestigationPage } from "@/features/alert-investigation/AlertInvestigationPage";
import { AlertInventoryPage } from "@/features/alert-inventory/AlertInventoryPage";
import { DataSourcesPage } from "@/features/data-sources/DataSourcesPage";
import { DetectionsPage } from "@/features/detections/DetectionsPage";
import { NotFoundPage } from "./NotFoundPage";
import { RootErrorBoundary } from "./RootErrorBoundary";

/**
 * `/alerts` is the product's primary entry point (the alert inventory);
 * `/` redirects there. `/alerts/:alertId` remains the full Dark Evidence
 * Map investigation screen, reached by selecting a row in the inventory.
 * An unmatched path falls through to the catch-all not-found route,
 * distinct from `/alerts/:alertId`'s own "no alert exists with this id"
 * data-layer state.
 *
 * Exported separately from `router` so tests can mount this exact route
 * config under `createMemoryRouter` with a controlled initial entry,
 * rather than duplicating it.
 */
export const routes: RouteObject[] = [
  {
    path: "/",
    element: <Navigate to="/alerts" replace />,
  },
  {
    path: "/alerts",
    element: <AlertInventoryPage />,
    errorElement: <RootErrorBoundary />,
  },
  {
    path: "/alerts/:alertId",
    element: <AlertInvestigationPage />,
    errorElement: <RootErrorBoundary />,
  },
  {
    path: "/data-sources",
    element: <DataSourcesPage />,
    errorElement: <RootErrorBoundary />,
  },
  {
    path: "/detections",
    element: <DetectionsPage />,
    errorElement: <RootErrorBoundary />,
  },
  {
    path: "*",
    element: <NotFoundPage />,
  },
];

export const router = createBrowserRouter(routes);
