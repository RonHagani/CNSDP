import { describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { mockJsonResponse, stubFetchRoutes } from "@/test/mockFetch";
import { DetectionsPage } from "./DetectionsPage";

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

const THREE_DETECTIONS = [scenario1, scenario2, scenario3];

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/detections"]}>
        <DetectionsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("DetectionsPage", () => {
  it("requests GET /v1/detections through the same-origin /api proxy prefix", async () => {
    const fetchMock = stubFetchRoutes({
      "/v1/detections": () => mockJsonResponse(200, { detections: THREE_DETECTIONS, total: 3 }),
    });
    renderPage();
    await screen.findByText("3 detections");
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/detections", expect.anything());
  });

  it("renders a loading state before the detections resolve", () => {
    stubFetchRoutes({ "/v1/detections": () => new Promise<Response>(() => {}) });
    renderPage();
    expect(screen.getByText(/Loading detections/)).toBeInTheDocument();
  });

  it("renders an unauthorized state for a backend 401", async () => {
    stubFetchRoutes({ "/v1/detections": () => mockJsonResponse(401, {}) });
    renderPage();
    await screen.findByText("Authentication required");
  });

  it("renders an error state with a retry action for a 5xx", async () => {
    stubFetchRoutes({ "/v1/detections": () => mockJsonResponse(500, {}) });
    renderPage();
    await screen.findByText(/The detections backend could not be reached/);
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("retry performs a real new HTTP request", async () => {
    const user = userEvent.setup();
    const fetchMock = stubFetchRoutes({ "/v1/detections": () => mockJsonResponse(500, {}) });
    renderPage();
    await screen.findByText(/The detections backend could not be reached/);
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Retry" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
  });

  it("renders the empty-result case honestly rather than fabricating a card", async () => {
    stubFetchRoutes({
      "/v1/detections": () => mockJsonResponse(200, { detections: [], total: 0 }),
    });
    renderPage();
    await screen.findByText("No active detections");
  });

  it("renders one card per active definition with name, scenario id, description, and revision", async () => {
    stubFetchRoutes({
      "/v1/detections": () => mockJsonResponse(200, { detections: THREE_DETECTIONS, total: 3 }),
    });
    renderPage();
    await screen.findByText("Interactive container exec request");

    expect(screen.getByText("scenario-1")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Detection of a Kubernetes API request to the pods/exec subresource that exhibits documented interactive-execution characteristics.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(/rev\. 111122223333…/)).toBeInTheDocument();

    expect(screen.getByText("High-risk Pod creation")).toBeInTheDocument();
    expect(screen.getByText("Cluster-admin ClusterRoleBinding grant")).toBeInTheDocument();
  });

  it("renders the declared condition structure faithfully: operation, requires_outcome, and characteristic descriptions", async () => {
    stubFetchRoutes({
      "/v1/detections": () => mockJsonResponse(200, { detections: [scenario2], total: 1 }),
    });
    renderPage();
    await screen.findByText("High-risk Pod creation");

    expect(screen.getByText("pods · create")).toBeInTheDocument();
    expect(screen.getByText("success")).toBeInTheDocument();
    expect(screen.getByText("A container in the Pod requests privileged mode.")).toBeInTheDocument();
    expect(screen.getByText("The Pod uses the host network.")).toBeInTheDocument();
  });

  it("renders a subresource-qualified operation for scenario 1, which declares no verb", async () => {
    stubFetchRoutes({
      "/v1/detections": () => mockJsonResponse(200, { detections: [scenario1], total: 1 }),
    });
    renderPage();
    await screen.findByText("Interactive container exec request");
    expect(screen.getByText("pods/exec")).toBeInTheDocument();
  });
});

describe("DetectionsPage — shell", () => {
  it("marks Detections as the active navigation destination, alongside the still-disabled System Health", async () => {
    stubFetchRoutes({
      "/v1/detections": () => mockJsonResponse(200, { detections: THREE_DETECTIONS, total: 3 }),
    });
    renderPage();
    await screen.findByRole("heading", { name: "Detections" });

    const detectionsLink = screen.getByRole("link", { name: "Detections" });
    expect(detectionsLink).toHaveAttribute("aria-current", "page");

    const systemHealth = screen.getByText("System Health").closest("span");
    expect(systemHealth).toHaveAttribute("aria-disabled", "true");
  });
});
