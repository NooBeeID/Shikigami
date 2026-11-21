import type { AppConfig } from './types';

function bridge(): any {
  return (window as any).go?.main?.Bridge;
}

export async function getAppConfig(): Promise<AppConfig> {
  try {
    const fn = bridge()?.AppConfigGet;
    if (typeof fn !== 'function') return { zoom: 100 };
    return (await fn.call(bridge())) as AppConfig;
  } catch {
    return { zoom: 100 };
  }
}

export async function setAppConfig(patch: Partial<AppConfig>): Promise<AppConfig> {
  try {
    const fn = bridge()?.AppConfigSet;
    if (typeof fn !== 'function') return { zoom: 100, ...patch };
    return (await fn.call(bridge(), patch)) as AppConfig;
  } catch {
    return { zoom: 100, ...patch };
  }
}

export async function pickDirectory(): Promise<string | null> {
  try {
    const fn = bridge()?.OpenDirectoryDialog;
    if (typeof fn !== 'function') return null;
    const r = await fn.call(bridge());
    return typeof r === 'string' && r !== '' ? r : null;
  } catch {
    return null;
  }
}
