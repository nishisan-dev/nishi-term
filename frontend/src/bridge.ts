export type Config = {
  host: string;
  port: number;
  user: string;
  password: string;
  authMethod?: "password" | "key" | "vault";
  savedCredential?: string;
  privateKeyPath?: string;
  passphrase?: string;
  knownHosts: string;
  cols: number;
  rows: number;
};
export type HostChallenge = {
  token: string;
  endpoint: string;
  algorithm: string;
  fingerprint: string;
};
export type Frame = {
  trust?: HostChallenge;
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
export type VaultStatus = { exists: boolean; locked: boolean };
export type RemoteDirectory = {
  path: string;
  entries: {
    name: string;
    size: number;
    modified: number;
    permissions: string;
    directory: boolean;
    symlink: boolean;
  }[];
};
export interface Bridge {
  ListFiles(id: string, path: string): Promise<RemoteDirectory>;
  CancelFiles(id: string): Promise<void>;
  SessionUsage(): Promise<number>;
  VaultStatus(): Promise<VaultStatus>;
  CreateVault(master: string): Promise<void>;
  UnlockVault(master: string): Promise<void>;
  LockVault(): Promise<void>;
  SaveCredential(id: string, config: Config): Promise<void>;
  DeleteCredential(id: string): Promise<void>;

  ListProfiles(): Promise<Profile[]>;
  SaveProfile(profile: Profile): Promise<Profile[]>;
  DeleteProfile(id: string): Promise<Profile[]>;
  ConfirmHost(id: string, token: string): Promise<void>;
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
