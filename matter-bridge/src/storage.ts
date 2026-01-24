import { existsSync, mkdirSync, readFileSync, writeFileSync } from "fs";
import { join } from "path";

export interface MatterStorage {
  fabricId?: string;
  nodeId?: string;
  commissioned: boolean;
  [key: string]: unknown;
}

export class StorageManager {
  private dataDir: string;
  private storagePath: string;
  private data: MatterStorage;

  constructor(dataDir: string = "./data") {
    this.dataDir = dataDir;
    this.storagePath = join(dataDir, "matter-state.json");
    this.data = { commissioned: false };
    this.ensureDataDir();
    this.load();
  }

  private ensureDataDir(): void {
    if (!existsSync(this.dataDir)) {
      mkdirSync(this.dataDir, { recursive: true });
    }
  }

  private load(): void {
    try {
      if (existsSync(this.storagePath)) {
        const content = readFileSync(this.storagePath, "utf-8");
        this.data = JSON.parse(content);
      }
    } catch (error) {
      console.error("Failed to load storage:", error);
      this.data = { commissioned: false };
    }
  }

  save(): void {
    try {
      writeFileSync(this.storagePath, JSON.stringify(this.data, null, 2));
    } catch (error) {
      console.error("Failed to save storage:", error);
    }
  }

  get<T>(key: string, defaultValue: T): T {
    return (this.data[key] as T) ?? defaultValue;
  }

  set<T>(key: string, value: T): void {
    this.data[key] = value;
    this.save();
  }

  isCommissioned(): boolean {
    return this.data.commissioned;
  }

  setCommissioned(commissioned: boolean, fabricId?: string, nodeId?: string): void {
    this.data.commissioned = commissioned;
    if (fabricId) this.data.fabricId = fabricId;
    if (nodeId) this.data.nodeId = nodeId;
    this.save();
  }

  getFabricId(): string | undefined {
    return this.data.fabricId;
  }

  getNodeId(): string | undefined {
    return this.data.nodeId;
  }

  getDataDir(): string {
    return this.dataDir;
  }

  clear(): void {
    this.data = { commissioned: false };
    this.save();
  }
}
