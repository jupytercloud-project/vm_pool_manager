import { apiFetch } from './api';

// Accès aux applications des VMs (JupyterLab, code-server) via le reverse-proxy HTTPS du
// control center — jamais l'IP directe de la VM (qui reste non exposée).
//
// Le navigateur ne peut pas porter le Bearer token sur une iframe / un nouvel onglet /
// un WebSocket. On demande donc d'abord une « session de proxy » (POST authentifié) qui
// pose un cookie HttpOnly, puis on ouvre l'URL renvoyée : le cookie voyage tout seul.

export type ProxyKind = 'jupyter' | 'vscode';
export type ProxyMode = 'read' | 'write';

export interface ProxyOpened {
  url: string;     // base du proxy (origin-relative), à ouvrir / mettre en iframe src
  mode: ProxyMode;
  target: string;
}

// openProxySession ouvre (et autorise côté serveur) une session de proxy vers une VM.
//   kind   : 'jupyter' | 'vscode'
//   poolId : serverpool_id ; ownerId : user_id propriétaire du pool
//   target : 'self' (sa VM) | 'instructor' | <email élève> (réservé au staff)
//   mode   : 'read' | 'write' (vscode)
// Renvoie l'URL à ouvrir, ou lève une erreur lisible (403/503…).
export async function openProxySession(
  kind: ProxyKind,
  poolId: string,
  ownerId: string,
  target: string = 'self',
  mode: ProxyMode = 'write',
): Promise<ProxyOpened> {
  const res = await apiFetch('/api/proxy-session', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ kind, pool_id: poolId, owner_id: ownerId, target, mode }),
  });
  if (!res.ok) {
    throw new Error((await res.text()) || `proxy-session ${res.status}`);
  }
  return res.json();
}

// rejoindre le VS Code d'un binôme via (cible + mot de passe). Pose le cookie vscode et
// renvoie l'URL à ouvrir, dans le mode autorisé par le grant.
export async function joinVscode(
  poolId: string,
  ownerId: string,
  target: string,
  password: string,
): Promise<ProxyOpened> {
  const res = await apiFetch('/api/vscode-grant/join', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ pool_id: poolId, owner_id: ownerId, target, password }),
  });
  if (!res.ok) {
    throw new Error((await res.text()) || `join ${res.status}`);
  }
  return res.json();
}

// partager SON VS Code : crée un grant (mode + mot de passe + expiration).
export async function shareVscode(
  poolId: string,
  ownerId: string,
  mode: ProxyMode,
  password: string,
  ttlHours = 24,
): Promise<{ ok: boolean; target: string; mode: ProxyMode; expires_at: string }> {
  const res = await apiFetch('/api/vscode-grant', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ pool_id: poolId, owner_id: ownerId, mode, password, ttl_hours: ttlHours }),
  });
  if (!res.ok) {
    throw new Error((await res.text()) || `share ${res.status}`);
  }
  return res.json();
}

// ouvrir l'URL d'un proxy dans un nouvel onglet (Jupyter / VS Code plein écran).
export function openInNewTab(url: string) {
  window.open(url, '_blank', 'noopener');
}
