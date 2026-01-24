import { Endpoint } from "@matter/main";
import { ThermostatDevice, ThermostatRequirements } from "@matter/node/devices";
import { Thermostat } from "@matter/main/clusters";

export interface ThermostatState {
  deviceId: number;
  name: string;
  currentTemp: number;      // Celsius
  heatSetpoint: number;     // Celsius
  coolSetpoint: number;     // Celsius
  systemMode: string;       // "off", "heat", "cool", "auto"
  humidity: number;         // Percentage
  isHeating: boolean;
  isCooling: boolean;
}

export type CommandHandler = (action: string, value: unknown) => Promise<void>;

// Convert Celsius to Matter's 0.01°C units
function celsiusToMatter(celsius: number): number {
  return Math.round(celsius * 100);
}

// Convert Matter's 0.01°C units to Celsius
function matterToCelsius(matter: number): number {
  return matter / 100;
}

// Convert system mode string to Matter enum
function systemModeToMatter(mode: string): Thermostat.SystemMode {
  switch (mode) {
    case "heat":
      return Thermostat.SystemMode.Heat;
    case "cool":
      return Thermostat.SystemMode.Cool;
    case "auto":
      return Thermostat.SystemMode.Auto;
    case "off":
    default:
      return Thermostat.SystemMode.Off;
  }
}

// Convert Matter enum to system mode string
function matterToSystemMode(mode: Thermostat.SystemMode): string {
  switch (mode) {
    case Thermostat.SystemMode.Heat:
      return "heat";
    case Thermostat.SystemMode.Cool:
      return "cool";
    case Thermostat.SystemMode.Auto:
      return "auto";
    case Thermostat.SystemMode.Off:
    default:
      return "off";
  }
}

// Create a thermostat server with heating and cooling features
const ThermostatServerWithFeatures = ThermostatRequirements.ThermostatServer.with("Heating", "Cooling");

// Create the device type with thermostat behavior
const TccThermostatDevice = ThermostatDevice.with(ThermostatServerWithFeatures);

export class ThermostatEndpoint {
  private endpoint: Endpoint<typeof TccThermostatDevice>;
  private commandHandler?: CommandHandler;
  private currentState: ThermostatState;

  constructor(name: string = "TCC Thermostat") {
    this.currentState = {
      deviceId: 0,
      name: name,
      currentTemp: 20,
      heatSetpoint: 20,
      coolSetpoint: 24,
      systemMode: "off",
      humidity: 50,
      isHeating: false,
      isCooling: false,
    };

    // Create the thermostat endpoint with the device type and thermostat behavior
    this.endpoint = new Endpoint(
      TccThermostatDevice,
      {
        id: "thermostat",
        thermostat: {
          localTemperature: celsiusToMatter(this.currentState.currentTemp),
          occupiedHeatingSetpoint: celsiusToMatter(this.currentState.heatSetpoint),
          occupiedCoolingSetpoint: celsiusToMatter(this.currentState.coolSetpoint),
          systemMode: Thermostat.SystemMode.Off,
          controlSequenceOfOperation: Thermostat.ControlSequenceOfOperation.CoolingAndHeating,
          minHeatSetpointLimit: celsiusToMatter(10),
          maxHeatSetpointLimit: celsiusToMatter(32),
          minCoolSetpointLimit: celsiusToMatter(10),
          maxCoolSetpointLimit: celsiusToMatter(35),
        },
      }
    );
  }

  getEndpoint(): Endpoint<typeof TccThermostatDevice> {
    return this.endpoint;
  }

  setCommandHandler(handler: CommandHandler): void {
    this.commandHandler = handler;
  }

  async setupCommandHandlers(): Promise<void> {
    // Watch for attribute changes from HomeKit
    this.endpoint.events.thermostat.occupiedHeatingSetpoint$Changed.on(async (value: number) => {
      if (this.commandHandler) {
        await this.commandHandler("setHeatingSetpoint", matterToCelsius(value));
      }
    });

    this.endpoint.events.thermostat.occupiedCoolingSetpoint$Changed.on(async (value: number) => {
      if (this.commandHandler) {
        await this.commandHandler("setCoolingSetpoint", matterToCelsius(value));
      }
    });

    this.endpoint.events.thermostat.systemMode$Changed.on(async (value: Thermostat.SystemMode) => {
      if (this.commandHandler) {
        await this.commandHandler("setSystemMode", matterToSystemMode(value));
      }
    });
  }

  async updateState(state: ThermostatState): Promise<void> {
    this.currentState = state;

    try {
      await this.endpoint.set({
        thermostat: {
          localTemperature: celsiusToMatter(state.currentTemp),
          occupiedHeatingSetpoint: celsiusToMatter(state.heatSetpoint),
          occupiedCoolingSetpoint: celsiusToMatter(state.coolSetpoint),
          systemMode: systemModeToMatter(state.systemMode),
        },
      });
    } catch (error) {
      console.error("Failed to update thermostat state:", error);
    }
  }

  getState(): ThermostatState {
    return this.currentState;
  }
}
