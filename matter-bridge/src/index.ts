import "@matter/nodejs";
import { ServerNode, VendorId } from "@matter/main";
import { ThermostatEndpoint, ThermostatState } from "./thermostat.js";
import { BridgeServer } from "./server.js";
import { StorageManager } from "./storage.js";

const VENDOR_ID = VendorId(0xFFF1); // Test vendor ID
const PRODUCT_ID = 0x8001;
const DEVICE_NAME = "TCC Thermostat";
const PORT = parseInt(process.env.MATTER_PORT || "5540", 10);

class MatterBridge {
  private server?: ServerNode;
  private thermostat: ThermostatEndpoint;
  private bridgeServer: BridgeServer;
  private storage: StorageManager;

  constructor() {
    this.storage = new StorageManager("./data");
    this.thermostat = new ThermostatEndpoint(DEVICE_NAME);
    this.bridgeServer = new BridgeServer(PORT);

    // Set up state update handler
    this.bridgeServer.setStateHandler(async (state: ThermostatState) => {
      await this.thermostat.updateState(state);
    });

    // Set up command handler
    this.thermostat.setCommandHandler(async (action: string, value: unknown) => {
      console.log(`Command received: ${action} = ${value}`);
      this.bridgeServer.broadcastCommand(action, value);
    });

    // Set up decommission handler
    this.bridgeServer.setDecommissionHandler(async () => {
      await this.decommission();
    });
  }

  async start(): Promise<void> {
    console.log("Starting Matter Bridge...");

    // Start HTTP/WebSocket server
    await this.bridgeServer.start();

    // Create the Matter server
    this.server = await ServerNode.create({
      id: "tcc-matter-bridge",

      // Basic information about this device
      productDescription: {
        name: DEVICE_NAME,
        deviceType: 0x0301, // Thermostat device type
      },

      // Commissioning options
      basicInformation: {
        vendorId: VENDOR_ID,
        vendorName: "TCC Bridge",
        productId: PRODUCT_ID,
        productName: DEVICE_NAME,
        productLabel: DEVICE_NAME,
        serialNumber: "TCC-001",
        hardwareVersion: 1,
        softwareVersion: 1,
        hardwareVersionString: "1.0",
        softwareVersionString: "1.0.0",
      },

      // Commissioning settings
      commissioning: {
        passcode: 20202021,
        discriminator: 3840,
      },

      // Network settings
      network: {
        port: PORT,
      },
    });

    // Add the thermostat endpoint
    await this.server.add(this.thermostat.getEndpoint());

    // Set up commissioning event handlers
    this.server.lifecycle.commissioned.on(() => {
      console.log("Device commissioned!");
      this.storage.setCommissioned(true);
      this.bridgeServer.setCommissioned(true);
    });

    this.server.lifecycle.decommissioned.on(() => {
      console.log("Device decommissioned");
      this.storage.setCommissioned(false);
      this.bridgeServer.setCommissioned(false);
    });

    // Start the Matter server
    await this.server.start();

    // Check if already commissioned (from previous session)
    // Matter.js persists fabric state, so on restart we need to sync our status
    const isCommissioned = this.server.state.commissioning.commissioned;
    if (isCommissioned) {
      console.log("Device already commissioned (restored from previous session)");
      this.storage.setCommissioned(true);
      this.bridgeServer.setCommissioned(true);
    }

    // Get and broadcast pairing information
    const qrCode = this.server.state.commissioning.pairingCodes.qrPairingCode;
    const manualPairCode = this.server.state.commissioning.pairingCodes.manualPairingCode;

    console.log("\n==============================================");
    console.log("Matter Thermostat Device Started");
    console.log("==============================================");
    console.log(`QR Code: ${qrCode}`);
    console.log(`Manual Pairing Code: ${manualPairCode}`);
    console.log("==============================================\n");

    this.bridgeServer.setPairingInfo(qrCode, manualPairCode);

    // Set up thermostat command handlers
    await this.thermostat.setupCommandHandlers();

    console.log("Matter Bridge ready!");
  }

  async decommission(): Promise<void> {
    console.log("Decommissioning Matter device...");

    if (!this.server) {
      throw new Error("Server not initialized");
    }

    // Erase the Matter server (factory reset)
    await this.server.erase();

    // Clear storage
    this.storage.clear();

    // Update bridge server status
    this.bridgeServer.setCommissioned(false);

    console.log("Matter device decommissioned successfully");
  }

  async stop(): Promise<void> {
    console.log("Stopping Matter Bridge...");

    if (this.server) {
      await this.server.close();
    }

    await this.bridgeServer.stop();

    console.log("Matter Bridge stopped");
  }
}

// Main entry point
const bridge = new MatterBridge();

// Handle shutdown signals
process.on("SIGINT", async () => {
  console.log("\nReceived SIGINT, shutting down...");
  await bridge.stop();
  process.exit(0);
});

process.on("SIGTERM", async () => {
  console.log("\nReceived SIGTERM, shutting down...");
  await bridge.stop();
  process.exit(0);
});

// Start the bridge
bridge.start().catch((error) => {
  console.error("Failed to start Matter Bridge:", error);
  process.exit(1);
});
