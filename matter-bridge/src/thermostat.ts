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
  private isUpdating: boolean = false;

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
      if (this.commandHandler && !this.isUpdating) {
        // Update our cached state so we don't try to re-set this value
        this.currentState.heatSetpoint = matterToCelsius(value);
        await this.commandHandler("setHeatingSetpoint", matterToCelsius(value));
      }
    });

    this.endpoint.events.thermostat.occupiedCoolingSetpoint$Changed.on(async (value: number) => {
      if (this.commandHandler && !this.isUpdating) {
        // Update our cached state so we don't try to re-set this value
        this.currentState.coolSetpoint = matterToCelsius(value);
        await this.commandHandler("setCoolingSetpoint", matterToCelsius(value));
      }
    });

    this.endpoint.events.thermostat.systemMode$Changed.on(async (value: Thermostat.SystemMode) => {
      if (this.commandHandler && !this.isUpdating) {
        // Update our cached state so we don't try to re-set this value
        this.currentState.systemMode = matterToSystemMode(value);
        await this.commandHandler("setSystemMode", matterToSystemMode(value));
      }
    });
  }

  async updateState(state: ThermostatState): Promise<void> {
    const prevState = this.currentState;
    this.currentState = state;

    const prevTempF = (prevState.currentTemp * 9/5 + 32).toFixed(1);
    const newTempF = (state.currentTemp * 9/5 + 32).toFixed(1);
    console.log(`updateState called: prev temp=${prevTempF}°F (${prevState.currentTemp.toFixed(2)}°C), new temp=${newTempF}°F (${state.currentTemp.toFixed(2)}°C)`);

    try {
      // Set flag to prevent event handlers from firing during programmatic updates
      this.isUpdating = true;

      // Only update values that have actually changed to avoid triggering
      // Matter.js reactive loops
      const updates: Record<string, unknown> = {};

      const newLocalTemp = celsiusToMatter(state.currentTemp);
      const newHeatSetpoint = celsiusToMatter(state.heatSetpoint);
      const newCoolSetpoint = celsiusToMatter(state.coolSetpoint);
      const newSystemMode = systemModeToMatter(state.systemMode);

      const prevLocalTemp = celsiusToMatter(prevState.currentTemp);
      console.log(`Comparing localTemperature: prev=${prevLocalTemp} (${prevState.currentTemp.toFixed(2)}°C), new=${newLocalTemp} (${state.currentTemp.toFixed(2)}°C), different=${prevLocalTemp !== newLocalTemp}`);

      if (prevLocalTemp !== newLocalTemp) {
        updates.localTemperature = newLocalTemp;
      }
      if (celsiusToMatter(prevState.heatSetpoint) !== newHeatSetpoint) {
        updates.occupiedHeatingSetpoint = newHeatSetpoint;
      }
      if (celsiusToMatter(prevState.coolSetpoint) !== newCoolSetpoint) {
        updates.occupiedCoolingSetpoint = newCoolSetpoint;
      }
      if (systemModeToMatter(prevState.systemMode) !== newSystemMode) {
        updates.systemMode = newSystemMode;
      }

      console.log(`Changes detected: ${Object.keys(updates).length > 0 ? Object.keys(updates).join(', ') : 'none'}`);

      // Only call set() if there are actual changes
      if (Object.keys(updates).length > 0) {
        // Log the update with key thermostat values
        const tempF = (state.currentTemp * 9/5 + 32).toFixed(1);
        const heatF = (state.heatSetpoint * 9/5 + 32).toFixed(1);
        const coolF = (state.coolSetpoint * 9/5 + 32).toFixed(1);
        console.log(`Publishing to Matter: temp=${tempF}°F, heat=${heatF}°F, cool=${coolF}°F, mode=${state.systemMode}`);

        await this.endpoint.set({
          thermostat: updates,
        });

        console.log("Matter state update successful");
      } else {
        console.log("No changes to publish to Matter");
      }
    } catch (error) {
      console.error("Failed to update thermostat state:", error);
    } finally {
      // Always clear the flag, even if update failed
      this.isUpdating = false;
    }
  }

  getState(): ThermostatState {
    return this.currentState;
  }
}
