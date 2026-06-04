# Création des pools

Un **serverpool** est un groupe de VMs identiques (même image, flavor, réseau) destiné aux
étudiants d'un cours. Sa création part du formulaire frontend et descend jusqu'à OpenStack.

## Vue d'ensemble

```mermaid
sequenceDiagram
    autonumber
    participant T as Enseignant
    participant FE as Frontend (CreateServerPoolModal)
    participant CC as Control Center (PoolService)
    participant PG as PostgreSQL
    participant MS as Microservice (PoolManager)
    participant SQ as SQLite (jobs)
    participant OS as OpenStack (vmpool)

    T->>FE: Remplit le formulaire (nom, image, flavor, réseau, min/max, port, planning)
    FE->>CC: CreatePool RPC (CreatePoolRequest)
    CC->>PG: INSERT serverpool (status=creating)
    CC->>CC: récupère le config_data (script d'init) — préfère la ligne 'system'
    CC->>MS: SendRessources(Status=CREATE, Type=SERVERPOOL, Data=poolData)
    MS->>SQ: stocke le pool + config
    CC->>PG: UPDATE status=running
    CC->>FE: CreatePoolResponse(success)
    loop Réconciliateur (toutes les 2 s)
        MS->>MS: count VMs < MinVM ? → enqueue CreateVM jobs
        MS->>OS: servers.Create(image résolu, flavor, réseau, cloud-init)
        OS-->>MS: VM ACTIVE
    end
    OS->>CC: au boot, la VM POST /api/register (vm-registrar)
    Note over CC,FE: PostgreSQL NOTIFY → le frontend voit les VMs apparaître
```

## Le formulaire (frontend)

`frontend/src/lib/components/CreateServerPoolModal.svelte` — sections :

1. **Général** — nom, Min/Max VMs, **port application** (optionnel : `0` = aucune app web,
   sinon `8888` pour Jupyter). ⚠️ Mettre **0** pour une VM sans app (Ubuntu) sinon l'étudiant
   verrait un « Démarrage de l'application… » qui tourne dans le vide.
2. **Infrastructure** — Système d'exploitation (famille → version) **ou** environnement Jupyter
   (les images `jupyter-snapshot-*` regroupées sous « JupyterHub », port 8888 auto + config
   d'autostart `jupyter-snapshot-<suffixe>` sélectionnée automatiquement) ; réseau.
3. **Flavor** — liste des flavors avec un statut calculé selon le disque requis par l'image :
   `★ Recommandé` / `ok` / `✗ Disque insuffisant`. Pour les snapshots Jupyter (qui n'exposent
   pas `min_disk`), un défaut de **20 GB** est utilisé (`getImageDiskGb`).
4. **Options avancées** — script d'initialisation, **jours de fermeture** (VMs éteintes ces
   jours, cf. [Provisionnement](04-provisionnement-reconciliation.md#jours-de-fermeture-off-days)),
   **planning** (Jour/Heure/Durée).

La soumission appelle `handleCreateServerpool` (`frontend/src/routes/serverpool/[[id]]/+page.svelte`)
qui construit un `CreatePoolRequest` (user, name, image, flavor, network, min/max, config,
metadata `off_days`, planning, appPort).

## Côté Control Center

`control_center/internal/pool/service.go` → `CreatePool` :

```mermaid
flowchart TB
    A["CreatePool(req)"] --> B["models.Serverpool{...}\nstatus=creating"]
    B --> C["config.Database.Create(&pool)"]
    C --> D["poolData = pool.ToMap()"]
    D --> E{"ConfigID défini ?"}
    E -->|oui| F["SELECT config_pools WHERE name=ConfigID\nORDER BY user_id='system' d'abord\n→ poolData['config_data']"]
    E -->|non| G[" "]
    F --> H["pm.SendRessources(CREATE, SERVERPOOL, poolData)"]
    G --> H
    H --> I{"succès ?"}
    I -->|oui| J["UPDATE status=running"]
    I -->|non| K["UPDATE status=error"]
```

⚠️ **Config dupliquée** : plusieurs lignes peuvent partager un même `name` dans `config_pools`
(une ligne `system` maintenue par le script de snapshot + des copies par utilisateur). La requête
ordonne `user_id='system'` en premier pour **toujours** prendre la version canonique (sinon une
vieille copie périmée — p.ex. avec `start-notebook.sh` et un espace avant `#!/bin/bash` →
`Exec format error` au boot — serait choisie).

`pool.ToMap()` (`control_center/models/serverpool.go`) sérialise notamment `image_ref`,
`flavor_ref`, `min_vm`, `max_vm`, `config_id`, `off_days`, `networks`.

## Côté Microservice

`microservices/openstack/grpc/servergrpc.go` → `handleServerpool` (CREATE) crée un
`models.Serverpool` en SQLite (avec `OffDays = data["off_days"]`) et stocke le `config_data`
dans `config_pools`. Le **réconciliateur** prend ensuite le relais pour créer les VMs (voir
[Provisionnement](04-provisionnement-reconciliation.md)).

## Statuts d'un pool

```mermaid
stateDiagram-v2
    [*] --> creating: CreatePool
    creating --> running: SendRessources OK
    creating --> error: échec microservice
    running --> deleting: suppression / fin de fenêtre planifiée
    scheduled --> creating: début de fenêtre planifiée
    deleting --> [*]
```

## Suppression d'un pool

Bouton **Supprimer** → `DeletePool` RPC → le Control Center supprime le pool en base + envoie un
`SendRessources(DELETE)` au microservice, qui supprime les VMs OpenStack et ses lignes locales.
Le réconciliateur ne recrée pas (pool absent des deux bases).
