# Ajouts au rapport — Partie 3 (prêts à coller)

> Chaque bloc indique **où le placer**. Avec les styles de titre de Word, la
> numérotation se met à jour toute seule quand on insère une sous-partie au milieu.

---

## ① Phrase d'introduction
📍 **Tout en haut de « 3 — Déroulement détaillé des missions », juste avant 3.1.**

Cette partie retrace concrètement ce que j'ai construit durant le stage. Je l'ai organisée comme le projet s'est déroulé : d'abord les choix techniques qui ont tout conditionné, puis les outils d'infrastructure de mes débuts, et enfin le développement de VMPoolManager, mon projet principal, fonctionnalité par fonctionnalité. À chaque étape, j'ai cherché à expliquer non seulement *ce que* j'ai fait, mais surtout *pourquoi* — en gardant à l'esprit qu'une partie de ce travail doit rester compréhensible sans être informaticien.

---

## ② La préparation des images système (snapshots)
📍 **Nouvelle sous-partie juste après « 3.3.6 Intégration OpenStack ».**

Un pool n'a d'intérêt que si les machines arrivent déjà équipées du bon environnement de travail. Installer les logiciels à chaque démarrage serait lent et fragile ; j'ai donc préparé en amont des **images système** prêtes à l'emploi, appelées *snapshots*. Chaque cours a ses propres besoins, et j'ai construit une **quinzaine d'environnements** distincts : Python scientifique (scipy), Data Science (Python, R, Julia), Julia, BIO583, ECO589, MEC431, MEC558, etc.

Concrètement, pour chaque cours, je pars de l'image de référence fournie par l'IDCS et j'y ajoute **nbgrader** (l'outil de distribution et de correction des devoirs), le tout sous un nom unique et universel — ainsi, le script de démarrage est strictement identique sur toutes les machines, quel que soit le cours. Pour l'enseignant, j'ai préparé un snapshot dédié contenant JupyterLab, nbgrader et toute l'arborescence de cours.

La principale difficulté a été une contrainte de **quota** : le cloud ne me laissait construire qu'une machine à la fois. J'ai donc écrit un programme de construction « patient » qui attend que les ressources se libèrent, enchaîne les images une par une, retient sa progression (il peut être relancé sans tout recommencer) et saute les snapshots déjà construits. Résultat : quand un professeur crée un pool, ses machines démarrent **déjà prêtes**, identiques pour tous, en quelques minutes.

---

## ③ Intégration Moodle
📍 **Nouvelle sous-partie juste après « la correction des devoirs ».**
⚠️ Retire le paragraphe Moodle de la sous-partie « correction des devoirs » pour éviter le doublon.
🖼️ Insérer la figure **`fig-moodle.svg`**.

Moodle est la plateforme pédagogique de l'École : c'est là que vivent les cours, les inscriptions et le carnet de notes. J'ai connecté VMPoolManager à Moodle via ses **Web Services** (une API sécurisée par jeton). Quatre échanges structurent cette intégration :

1. **Connexion de l'élève** : l'étudiant se connecte avec ses identifiants Moodle. Aucune clé SSH n'est alors nécessaire — l'application crée une session et l'accès se fait dans le navigateur.
2. **Import des élèves** : à partir d'un cours Moodle, j'importe la liste des inscrits dans un pool en un clic. L'adresse e-mail sert de fil conducteur (cf. le modèle de données).
3. **Cours et devoirs** : la plateforme lit les cours et les devoirs disponibles, pour pouvoir les associer à un pool.
4. **Remontée des notes** : les notes calculées par nbgrader peuvent être renvoyées dans le carnet de notes Moodle.

Pour développer sans attendre les accès au Moodle de l'établissement, j'ai monté un **Moodle de test** local (via Docker) reproduisant des cours et des élèves. Le passage au vrai Moodle de l'École ne demandera que de changer l'adresse et le jeton. Comme indiqué plus haut, l'écriture des notes (point 4) restera **désactivée au déploiement** dans un premier temps, le temps de valider les accès.

---

## ④ Robustesse et fiabilité
📍 **Nouvelle sous-partie à la fin de 3.3, juste après « Observabilité ».**

Une application qui pilote de vraies machines doit tenir debout même quand quelque chose tourne mal. J'ai donc cherché à rendre les services « increvables » : un incident isolé — une requête qui échoue, une tâche de fond qui plante — ne doit jamais faire tomber tout le service. J'ai ajouté des **garde-fous** à chaque niveau (les requêtes web, les appels internes, et les tâches qui créent les machines) : en cas d'erreur imprévue, elle est journalisée et **isolée** plutôt que propagée. J'ai complété cela par des **tests automatisés** sur les fonctions sensibles (validation des entrées, lecture des notes, détection d'activité) et par un script de « pré-vol » qui vérifie, avant chaque démonstration, que tous les composants (base de données, cloud, services) répondent bien. C'est cette démarche qui m'a permis d'enchaîner plusieurs démonstrations sans le moindre incident.

---

## ⑤ Mini-conclusion de la partie 3
📍 **Tout à la fin de 3.3, juste avant « 4 — Difficultés ».**

Au terme de cette phase, VMPoolManager couvre l'ensemble du cycle pédagogique : créer un pool, attribuer une machine à chaque étudiant, distribuer et corriger les devoirs, remonter les notes et superviser le tout en temps réel. Construire un tel système ne s'est toutefois pas fait sans embûches — la partie suivante revient sur les difficultés les plus marquantes que j'ai rencontrées, et sur la manière dont je les ai résolues.

---

## Rappels figures
- **`fig-securite-auth.svg`** → à insérer dans la section **4.4** (là où il reste le texte `[Figure : fig-securite-auth …]`).
- **`fig-moodle.svg`** → dans la nouvelle sous-partie **Intégration Moodle** (③).
- Alternatives textuelles de toutes les figures : voir `doc/figures/README.md`.
