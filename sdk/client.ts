// BizEngine View Client SDK — minimal skeleton for CTO to integrate with Svelte 5
import type { Views, DataRef, ViewSnapshotMsg, TableDiffMsg, ViewDiffMsg } from "./views";

type ViewKey = keyof Views;

interface SubscribeResult {
  params_hash: string;
  version: number;
}

interface ViewSubscription<K extends ViewKey> {
  key: K;
  params: Views[K]["params"];
  params_hash: string;
  version: number;
  refs: DataRef[];
}

/**
 * ArcanaClient provides typed view subscriptions with real-time updates.
 *
 * Usage:
 *   const client = new ArcanaClient("/api/v1");
 *   client.setWorkspace(wsID);
 *   const result = await client.subscribe("orders_list", { status: "active" });
 *   // Centrifugo delivers table_diff / view_diff — wire handleSnapshot/handleTableDiff/handleViewDiff
 */
export class ArcanaClient {
  private baseURL: string;
  private wsID: string = "";
  private subscriptions = new Map<string, ViewSubscription<ViewKey>>();

  constructor(baseURL: string) {
    this.baseURL = baseURL;
  }

  setWorkspace(wsID: string): void {
    this.wsID = wsID;
  }

  async subscribe<K extends ViewKey>(
    key: K,
    params: Views[K]["params"]
  ): Promise<SubscribeResult> {
    const resp = await fetch(
      `${this.baseURL}/workspaces/${this.wsID}/views/subscribe`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ data: { view: key, params } }),
      }
    );

    const json = await resp.json();
    if (!json.ok) throw new Error(json.error?.details ?? "subscribe failed");

    const result: SubscribeResult = json.data;

    this.subscriptions.set(result.params_hash, {
      key,
      params,
      params_hash: result.params_hash,
      version: result.version,
      refs: [],
    });

    return result;
  }

  async unsubscribe<K extends ViewKey>(
    key: K,
    paramsHash: string
  ): Promise<void> {
    await fetch(
      `${this.baseURL}/workspaces/${this.wsID}/views/unsubscribe`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ data: { view: key, params_hash: paramsHash } }),
      }
    );

    this.subscriptions.delete(paramsHash);
  }

  async active(): Promise<{ view: string; params_hash: string; version: number }[]> {
    const resp = await fetch(
      `${this.baseURL}/workspaces/${this.wsID}/views/active`,
      { credentials: "include" }
    );

    const json = await resp.json();
    return json.data?.views ?? [];
  }

  async sync(
    views: { view: string; params_hash: string; version: number }[],
    tables: Record<string, Record<string, number>>
  ): Promise<void> {
    await fetch(
      `${this.baseURL}/workspaces/${this.wsID}/views/sync`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ data: { views, tables } }),
      }
    );
  }

  // --- Centrifugo message handlers (wire to your Centrifugo client) ---

  handleSnapshot(msg: ViewSnapshotMsg): void {
    const sub = this.subscriptions.get(msg.params_hash);
    if (!sub) return;
    sub.refs = msg.refs;
    sub.version = msg.version;
    // TODO: CTO — merge msg.tables into Svelte tableStore (replace, not merge)
  }

  handleTableDiff(msg: TableDiffMsg): void {
    // TODO: CTO — apply msg.patch to tableStore[msg.table][msg.id]
    // Svelte 5 proxies will auto-update all views referencing this row
    void msg;
  }

  handleViewDiff(msg: ViewDiffMsg): void {
    const sub = this.subscriptions.get(msg.params_hash);
    if (!sub) return;
    sub.version = msg.version;

    for (const op of msg.refs_patch) {
      if (op.op === "add" && op.value) {
        sub.refs.push(op.value as DataRef);
      } else if (op.op === "replace" && op.value === null) {
        const idx = parseInt(op.path.replace("/", ""), 10);
        if (!isNaN(idx) && idx < sub.refs.length) {
          sub.refs.splice(idx, 1);
        }
      }
    }
    // TODO: CTO — merge msg.tables into Svelte tableStore for new records
  }
}
