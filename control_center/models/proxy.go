package models

import "time"

// ProxySession = autorisation éphémère d'accès à un reverse-proxy applicatif
// (JupyterLab / code-server). Émise par un endpoint authentifié au Bearer, elle est
// référencée par un cookie HttpOnly que le navigateur renvoie automatiquement sur
// l'iframe, les liens et les WebSockets — là où le token JS ne peut pas voyager.
//
// La VM cible (TargetIP/TargetPort) est résolue CÔTÉ SERVEUR au moment de l'émission :
// le client ne choisit jamais une IP. C'est la défense anti-IDOR du dispositif.
type ProxySession struct {
	ID         string `gorm:"primaryKey"` // valeur opaque du cookie (aléatoire, non devinable)
	Email      string `gorm:"index"`      // identité qui a ouvert la session
	Kind       string // "jupyter" | "vscode"
	PoolID     string // serverpool_id (pour matcher le chemin du proxy)
	OwnerID    string // user_id propriétaire du pool
	Target     string // identité de la VM cible (email élève, ou owner pour la VM instructeur)
	VMID       string `gorm:"index"` // UUID de la VM (= chemin du proxy, URL-safe ; évite l'email dans l'URL)
	TargetIP   string // IP résolue côté serveur — jamais fournie par le client
	TargetPort int    // port résolu (8888 Jupyter, 8443 code-server RW, 8444 code-server RO)
	Mode       string // "read" | "write"
	ExpiresAt  time.Time
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}

// VscodeGrant = partage explicite, par une cible, de l'accès à SON VS Code (code-server).
// Un autre utilisateur présente (cible + mot de passe) ; si un grant valide existe, le
// proxy ouvre une ProxySession vers la VM de la cible dans le mode autorisé.
//
// Le prof n'a pas besoin de grant : son rôle (staff + supervision du pool) suffit.
type VscodeGrant struct {
	ID           uint      `gorm:"primaryKey;autoIncrement"`
	PoolID       string    `gorm:"index"` // serverpool_id du pool partagé
	OwnerID      string    // user_id propriétaire du pool
	Target       string    `gorm:"index"` // identité (email) de l'élève qui partage SA VM
	PasswordHash string    // bcrypt du mot de passe de partage
	Mode         string    // "read" | "write"
	ExpiresAt    time.Time // expiration du partage
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}
