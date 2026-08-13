import { test, expect } from "@playwright/test";
import { mockDetectionsBody, mockDetectionsStatus } from "./support/mockApi";
import type { DetectionsResponse } from "../src/types/contract";

/**
 * Real-browser coverage for `/detections` — a read-only catalog of the
 * platform's currently active detection definitions (FR-020, FR-021,
 * FR-022). The production frontend calls the real `GET /v1/detections`
 * endpoint (src/lib/api/client.ts, proxied same-origin by Vite — see
 * vite.config.ts); `mockDetectionsBody`/`mockDetectionsStatus`
 * (e2e/support/mockApi.ts) stand in for that backend at the network layer,
 * mirroring alert-inventory.spec.ts's own pattern.
 *
 * Content here mirrors the three real, committed scenario definitions
 * (definitions/scenario-{1,2,3}.yaml) for parity with real backend output,
 * the same convention DetectionsPage.test.tsx already uses. Detailed
 * rendering logic (condition-structure formatting, operation formatting) is
 * already covered at the unit level; this suite only proves what jsdom
 * cannot: real routing, real layout, and real browser-rendered states.
 */

const scenario1 = {
  scenario: "scenario-1",
  name: "Interactive container exec request",
  description:
    "Detection of a Kubernetes API request to the pods/exec subresource that exhibits documented interactive-execution characteristics.",
  revision: "1111222233334444555566667777888899990000aaaabbbbccccddddeeeeff",
  conditions: {
    operation: { resource: "pods", subresource: "exec" },
    requires_any: [
      { id: "stdin_streaming", description: "The exec request enables standard-input streaming." },
      { id: "tty_allocation", description: "The exec request requests interactive terminal (TTY) allocation." },
    ],
  },
};

const scenario2 = {
  scenario: "scenario-2",
  name: "High-risk Pod creation",
  description:
    "Detection of the creation of a Pod whose specification includes at least one documented high-risk privilege or host-access characteristic.",
  revision: "2222333344445555666677778888999900001111aaaabbbbccccddddeeeeff",
  conditions: {
    operation: { resource: "pods", verb: "create" },
    requires_outcome: "success",
    requires_any: [
      { id: "privileged_container", description: "A container in the Pod requests privileged mode." },
      { id: "host_network", description: "The Pod uses the host network." },
    ],
  },
};

const scenario3 = {
  scenario: "scenario-3",
  name: "Cluster-admin ClusterRoleBinding grant",
  description: "Detection of the creation of a ClusterRoleBinding that references the cluster-admin ClusterRole.",
  revision: "3333444455556666777788889999000011112222aaaabbbbccccddddeeeeff",
  conditions: {
    operation: { resource: "clusterrolebindings", verb: "create" },
    requires_outcome: "success",
    requires_all: [
      {
        id: "role_ref_cluster_admin",
        description: "The ClusterRoleBinding's role reference is the cluster-admin ClusterRole.",
      },
    ],
  },
};

const THREE_DETECTIONS: DetectionsResponse = { detections: [scenario1, scenario2, scenario3], total: 3 };

test.describe("Detections — catalog", () => {
  test("renders one card per active definition with real backend content", async ({ page }) => {
    await mockDetectionsBody(page, THREE_DETECTIONS);
    await page.goto("/detections");

    await expect(page.getByRole("heading", { name: "Detections" })).toBeVisible();
    await expect(page.getByText("3 detections")).toBeVisible();
    await expect(page.getByText("Interactive container exec request")).toBeVisible();
    await expect(page.getByText("High-risk Pod creation")).toBeVisible();
    await expect(page.getByText("Cluster-admin ClusterRoleBinding grant")).toBeVisible();
  });

  test("renders the empty-result case honestly, with no fabricated card", async ({ page }) => {
    await mockDetectionsBody(page, { detections: [], total: 0 });
    await page.goto("/detections");
    await expect(page.getByText("No active detections")).toBeVisible();
  });

  test("is reachable from the primary navigation and is marked the active destination", async ({ page }) => {
    await mockDetectionsBody(page, THREE_DETECTIONS);
    await page.goto("/alerts");

    await page.getByRole("link", { name: "Detections" }).click();

    await expect(page).toHaveURL(/\/detections$/);
    await expect(page.getByRole("heading", { name: "Detections" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Detections" })).toHaveAttribute("aria-current", "page");
  });

  test("the loading state is reachable before the response resolves", async ({ page }) => {
    await page.route("**/api/v1/detections", () => new Promise(() => {})); // never resolves
    await page.goto("/detections");
    await expect(page.getByText(/Loading detections/)).toBeVisible();
  });

  test("renders an unauthorized state for a 401", async ({ page }) => {
    await mockDetectionsStatus(page, 401);
    await page.goto("/detections");
    await expect(page.getByText("Authentication required")).toBeVisible();
  });

  test("the error state renders with a working retry action for a 5xx", async ({ page }) => {
    await mockDetectionsStatus(page, 500);
    await page.goto("/detections");
    await expect(page.getByText(/The detections backend could not be reached/)).toBeVisible();

    const retry = page.getByRole("button", { name: "Retry" });
    await expect(retry).toBeVisible();
    await retry.click();
    await expect(page.getByText(/The detections backend could not be reached/)).toBeVisible();
  });

  test("renders without horizontal page overflow at a standard viewport", async ({ page }) => {
    await mockDetectionsBody(page, THREE_DETECTIONS);
    await page.setViewportSize({ width: 1024, height: 900 });
    await page.goto("/detections");

    await expect(page.getByRole("heading", { name: "Detections" })).toBeVisible();
    const hasOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    );
    expect(hasOverflow).toBe(false);
  });
});
