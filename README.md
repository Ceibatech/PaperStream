# PaperStream

Dépôt automatique des documents numérisés avec **PaperStream Capture** dans **MULCV**.
Le dépôt cloné **est** le dossier de travail du poste : tous les dossiers existent déjà,
rien n'est à créer à la main.

## Installer un poste

1. Cloner le dépôt **n'importe où**, par exemple dans Téléchargements :
   ```
   git clone https://github.com/Ceibatech/PaperStream.git
   ```
2. Obtenir le **secret du poste** auprès de l'administrateur, qui le lit sur le serveur :
   `cat /opt/ceiba/keycloak/secrets/mulcv-depot-poste1.secret` (ou `poste2`).
3. Double-cliquer sur **`installer.cmd`** (à la racine du dépôt cloné), taper `1` ou `2`,
   coller le secret. L'installateur **crée `C:\MULCV`** et tout son contenu, puis vérifie la
   connexion : la fenêtre doit finir par **« Connexion à MULCV vérifiée »**.
4. Essai sans scanner : double-cliquer sur **`C:\MULCV\tester-envoi.cmd`**, glisser un PDF
   dans la fenêtre. S'il porte un QR de dossier transféré, appuyer sur Entrée : MULCV le lit.
   Sans QR, taper l'identifiant d'un dossier déjà transféré (ex. `TEST-DOS-001`).
   Le PDF arrive dans MULCV et sa copie dans `C:\MULCV\numerisation\2-envoyes`.

## Ce que l'installateur crée : `C:\MULCV`

```
C:\MULCV\
├── tester-envoi.cmd            ← double-clic : envoie un PDF d'essai sans scanner
├── bin\
│   └── mulcv-depot.exe         ← programme lancé par PaperStream
├── config\
│   ├── postes\poste-1.json     ← modèles
│   ├── postes\poste-2.json
│   └── mulcv-depot.json        ← configuration du poste, avec le secret (lisible par l'opérateur seul)
├── numerisation\
│   ├── 1-a-envoyer\            ← PaperStream dépose ici les PDF
│   ├── 2-envoyes\              ← PDF bien arrivés dans MULCV (copie locale)
│   └── 3-erreurs\              ← PDF refusés + un .txt qui explique pourquoi
└── journal\
    └── mulcv-depot.log         ← une ligne par envoi
```

Le dépôt cloné, lui, contient : `installer.cmd` et `install.ps1` (l'installation), `poste\`
(tout ce qui est copié dans `C:\MULCV`), `admin\` (comptes Keycloak) et `src\` (code source).

## Régler PaperStream Capture

**Aucun nom de fichier à régler.** MULCV lit lui-même les QR déjà collés sur les cartons et
les dossiers (ceux générés par GeoArchiv) et range chaque PDF dans le bon dossier.

| Réglage du profil | Valeur |
|---|---|
| Séparation de documents | **Code-barres**, type **QR Code** |
| Page du code-barres | **la conserver** (ne pas supprimer la page séparatrice) : MULCV en a besoin |
| Nom du fichier | celui proposé par PaperStream (`CCM_…`, compteur…), peu importe |
| Format | PDF ou PDF/A, 25 Mo au plus par dossier |
| Dossier de sortie | `C:\MULCV\numerisation\1-a-envoyer` |
| Destination | **Other App** |
| Application Path | `C:\MULCV\bin\mulcv-depot.exe` |
| Parameters | `%f` |
| Work Folder | `C:\MULCV` |

### Ordre de numérisation

```
QR CARTON   2010-KOU-0201          ← fiche du carton (conseillée)
QR DOSSIER  2010-KOU-0201-001      ← 1re feuille du dossier
  pièces du dossier…
QR DOSSIER  2010-KOU-0201-002
  pièce 1
SÉPARATEUR  MULCV-PIECE            ← entre deux pièces d'un même dossier
  pièce 2
```

**Pièces** : la feuille « séparateur de pièce » (QR `MULCV-PIECE`, réutilisable) se place
entre deux pièces. Chaque pièce est rangée à part, numérotée `…-P01-…`, `…-P02-…` à la suite
des pièces déjà présentes : le nombre de pièces d'un dossier se lit dans Share. Sans
séparateur, tout le dossier forme une seule pièce.

**MULCV lit le QR de chaque page et découpe lui-même** : toutes les pages qui suivent
l'étiquette d'un dossier lui appartiennent, jusqu'à l'étiquette suivante ou la fiche d'un
carton. Un carton entier peut donc arriver en un seul fichier : un PDF est rangé par dossier.
Les versos blancs des fiches sont ignorés.

Le fichier entier est refusé (dans `3-erreurs`, avec le numéro de page dans le `.txt`) si une
étiquette est inconnue de MULCV, si des pages précèdent la première étiquette, ou si un
dossier n'appartient pas au carton scanné. Rien n'est alors déposé : corriger la pile et la
renumériser.

**Mode de couleur fixe obligatoire** (couleur ou niveaux de gris). En « couleur automatique »
avec un fichier par mode d'image, PaperStream regroupe les pages par couleur et perd leur
ordre : impossible de savoir à quel dossier elles appartiennent.

Variante si « Other App » n'est pas utilisé : destination **Folder** vers
`C:\MULCV\numerisation\1-a-envoyer`, et installer avec la surveillance :
`powershell -ExecutionPolicy Bypass -File install.ps1 -Poste 1 -Surveillance` (depuis le dépôt cloné).

## D'où ça part, où ça arrive

1. PaperStream crée par exemple `C:\MULCV\numerisation\1-a-envoyer\CCM_02112026_00002.pdf` et lance `bin\mulcv-depot.exe`.
2. Le programme s'identifie auprès de Keycloak avec le compte du poste (`mulcv-depot-poste1` ou `2`).
3. Il envoie le PDF à MULCV, qui lit le QR de la première page : `DOS-000456`.
4. MULCV le range dans **`Scan / Production / <zone> / <conteneur> / <carton> / DOS-000456`**, le dossier créé
   quand GeoArchiv a transféré le conteneur. L'OCR l'analyse, puis l'archiviste le valide.
5. Sur le poste, le PDF passe dans `numerisation\2-envoyes`, ou dans `numerisation\3-erreurs`
   avec la raison. Si MULCV ou le réseau est indisponible, il reste dans `1-a-envoyer` et
   repart au prochain envoi.

## Les deux postes

| Poste | Compte technique | Modèle |
|---|---|---|
| Poste 1 | `mulcv-depot-poste1` | `config\postes\poste-1.json` |
| Poste 2 | `mulcv-depot-poste2` | `config\postes\poste-2.json` |

Chaque poste a son propre compte : MULCV sait quel poste a déposé chaque document, et un poste
perdu se révoque sans arrêter l'autre.

## Mettre à jour un poste

Dans le dépôt cloné : `git pull`, puis double-clic sur `installer.cmd` et répondre **O** pour
garder la configuration. Le programme est remplacé ; la configuration et les fichiers de
`numerisation\` ne sont pas touchés.

## Administration et développement

- `admin\keycloak\` : définitions des comptes des postes. Ils sont créés sur le serveur par
  `provision_intake_clients.py` (dépôt `MULCV-search-service`) et doivent être
  **Collaborateurs** du site MULCV dans Share.
- `./build.sh` : tests et reconstruction de `poste\bin\mulcv-depot.exe` (Go 1.24.7). La CI refuse un
  exécutable qui ne correspond pas exactement aux sources de `src\`.
- Contrat de l'API : `docs/transfer-api.md` du dépôt `MULCV-search-service`.
