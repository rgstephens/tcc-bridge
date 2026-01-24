import express, { Express, Request, Response } from "express";
import { WebSocketServer, WebSocket } from "ws";
import { createServer, Server as HttpServer } from "http";
import { ThermostatState } from "./thermostat.js";

export interface ServerStatus {
  running: boolean;
  commissioned: boolean;
  fabricId?: string;
  nodeId?: string;
  connectedPeers: number;
  uptime: number;
  lastUpdate: string;
}

export interface PairingInfo {
  qr_code: string;
  manual_pair_code: string;
  setup_url?: string;
}

export interface MatterEvent {
  type: string;
  timestamp: string;
  data?: Record<string, unknown>;
}

export type StateUpdateHandler = (state: ThermostatState) => Promise<void>;

export class BridgeServer {
  private app: Express;
  private httpServer: HttpServer;
  private wss: WebSocketServer;
  private clients: Set<WebSocket> = new Set();
  private port: number;
  private startTime: Date;
  private stateHandler?: StateUpdateHandler;

  // Status fields
  private commissioned = false;
  private fabricId?: string;
  private nodeId?: string;
  private qrCode = "";
  private manualPairCode = "";
  private connectedPeers = 0;
  private matterReady = false;

  constructor(port: number = 5540) {
    this.port = port;
    this.startTime = new Date();
    this.app = express();
    this.app.use(express.json());

    this.httpServer = createServer(this.app);
    this.wss = new WebSocketServer({ server: this.httpServer, path: "/events" });

    this.setupRoutes();
    this.setupWebSocket();
  }

  private setupRoutes(): void {
    // Health check
    this.app.get("/health", (_req: Request, res: Response) => {
      res.json({ status: "ok" });
    });

    // Get status
    this.app.get("/status", (_req: Request, res: Response) => {
      const status: ServerStatus = {
        running: this.matterReady,
        commissioned: this.commissioned,
        fabricId: this.fabricId,
        nodeId: this.nodeId,
        connectedPeers: this.connectedPeers,
        uptime: Math.floor((Date.now() - this.startTime.getTime()) / 1000),
        lastUpdate: new Date().toISOString(),
      };
      res.json(status);
    });

    // Get pairing info
    this.app.get("/pairing", (_req: Request, res: Response) => {
      const info: PairingInfo = {
        qr_code: this.qrCode,
        manual_pair_code: this.manualPairCode,
      };
      res.json(info);
    });

    // Update thermostat state (from Go backend)
    this.app.post("/state", async (req: Request, res: Response) => {
      try {
        const state = req.body as ThermostatState;
        if (this.stateHandler) {
          await this.stateHandler(state);
        }
        res.json({ status: "ok" });
      } catch (error) {
        console.error("Failed to update state:", error);
        res.status(500).json({ error: "Failed to update state" });
      }
    });
  }

  private setupWebSocket(): void {
    this.wss.on("connection", (ws: WebSocket) => {
      console.log("WebSocket client connected");
      this.clients.add(ws);

      ws.on("close", () => {
        console.log("WebSocket client disconnected");
        this.clients.delete(ws);
      });

      ws.on("error", (error) => {
        console.error("WebSocket error:", error);
        this.clients.delete(ws);
      });
    });
  }

  setStateHandler(handler: StateUpdateHandler): void {
    this.stateHandler = handler;
  }

  setPairingInfo(qrCode: string, manualPairCode: string): void {
    this.qrCode = qrCode;
    this.manualPairCode = manualPairCode;
    this.matterReady = true;
  }

  setCommissioned(commissioned: boolean, fabricId?: string, nodeId?: string): void {
    this.commissioned = commissioned;
    this.fabricId = fabricId;
    this.nodeId = nodeId;

    this.broadcastEvent({
      type: "commissioned",
      timestamp: new Date().toISOString(),
      data: { commissioned, fabricId, nodeId },
    });
  }

  setConnectedPeers(count: number): void {
    this.connectedPeers = count;
  }

  broadcastEvent(event: MatterEvent): void {
    const message = JSON.stringify(event);
    for (const client of this.clients) {
      if (client.readyState === WebSocket.OPEN) {
        client.send(message);
      }
    }
  }

  broadcastCommand(action: string, value: unknown): void {
    this.broadcastEvent({
      type: "command",
      timestamp: new Date().toISOString(),
      data: { action, value },
    });
  }

  async start(): Promise<void> {
    return new Promise((resolve) => {
      this.httpServer.listen(this.port, () => {
        console.log(`Bridge server listening on port ${this.port}`);
        resolve();
      });
    });
  }

  async stop(): Promise<void> {
    return new Promise((resolve) => {
      // Close all WebSocket connections
      for (const client of this.clients) {
        client.close();
      }
      this.clients.clear();

      this.wss.close(() => {
        this.httpServer.close(() => {
          console.log("Bridge server stopped");
          resolve();
        });
      });
    });
  }
}
