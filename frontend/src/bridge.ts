export type Config = {
  host: string;
  port: number;
  user: string;
  password: string;
  authMethod?: "password" | "key";
  privateKeyPath?: string;
  passphrase?: string;
  knownHosts: string;
  cols: number;
  rows: number;
};
export type Frame = {
  sequence: number;
  data: string;
  state: string;
  error: string;
};
export type Profile = {
  id: string;
  name: string;
  group: string;
  host: string;
  port: number;
  user: string;
  knownHosts: string;
};
export interface Bridge {
  ListProfiles(): Promise<Profile[]>;
  SaveProfile(profile: Profile): Promise<Profile[]>;
  DeleteProfile(id: string): Promise<Profile[]>;
  Start(config: Config): Promise<string>;
  Poll(id: string, ack: number): Promise<Frame>;
  Input(id: string, data: string): Promise<void>;
  Resize(id: string, cols: number, rows: number): Promise<void>;
  Close(id: string): Promise<void>;
}
declare global {
  interface Window {
    go?: { main: { App: Bridge } };
  }
}
export function api(): Bridge {
  const bridge = window.go?.main.App;
  if (!bridge) throw new Error("Abra pelo aplicativo Wails para usar SSH.");
  return bridge;
}
