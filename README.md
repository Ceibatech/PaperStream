# PaperStream

Dépôt automatique des documents numérisés avec **PaperStream Capture** dans **MULCV**.
Le dépôt cloné **est** le dossier de travail du poste : tous les dossiers existent déjà,
rien n'est à créer à la main.

## Le dossier `C:\MULCV` après le clone

```
C:\MULCV\
├── installer.cmd               ← double-clic : configure ce poste (une seule fois)
├── tester-envoi.cmd            ← double-clic : envoie un PDF d'essai sans scanner
├── bin\
│   └── mulcv-depot.exe         ← programme lancé par PaperStream
├── config\
│   ├── postes\poste-1.json     ← modèles fournis
│   ├── postes\poste-2.json
│   └── mulcv-depot.json        ← créé par installer.cmd (contient le secret, jamais dans Git)
├── numerisation\
│   ├── 1-a-envoyer\            ← PaperStream dépose ici les PDF
│   ├── 2-envoyes\              ← PDF bien arrivés dans MULCV (copie locale)
│   └── 3-erreurs\              ← PDF refusés + un .txt qui explique pourquoi
├── journal\
│   └── mulcv-depot.log         ← une ligne par envoi
├── admin\                      ← réservé à l'administrateur (clients Keycloak)
└── src\                        ← code source du programme
```

## Installer un poste

1. Cloner **dans `C:\MULCV`** (le dossier ne doit pas déjà exister) :
   ```
   git clone https://github.com/Ceibatech/PaperStream.git C:\MULCV
   ```
2. Obtenir le **secret du poste** auprès de l'administrateur, qui le lit sur le serveur :
   `cat /opt/ceiba/keycloak/secrets/mulcv-depot-poste1.secret` (ou `poste2`).
3. Double-cliquer sur **`C:\MULCV\installer.cmd`**, taper `1` ou `2`, coller le secret.
   La fenêtre doit finir par **« Connexion à MULCV vérifiée »**.
4. Essai sans scanner : double-cliquer sur **`tester-envoi.cmd`**, glisser un PDF dans la
   fenêtre, taper un identifiant de dossier déjà transféré (ex. `TEST-DOS-001`).
   Le PDF arrive dans MULCV et sa copie dans `numerisation\2-envoyes`.

## Régler PaperStream Capture

| Réglage du profil | Valeur |
|---|---|
| Séparation de documents | **Code-barres**, type **QR Code** (QR collé sur chaque dossier) |
| Nom du fichier | **valeur du QR** uniquement (ex. `DOS-000456`), sans préfixe ni date |
| Format | PDF ou PDF/A, 25 Mo au plus par dossier |
| Dossier de sortie | `C:\MULCV\numerisation\1-a-envoyer` |
| Destination | **Other App** |
| Application Path | `C:\MULCV\bin\mulcv-depot.exe` |
| Parameters | `%f` |
| Work Folder | `C:\MULCV` |

**Le nom du fichier doit être l'identifiant du dossier** : c'est lui qui dit à MULCV où ranger
le PDF. Un nom de compteur (`0001.pdf`) est refusé et part dans `3-erreurs`. Un suffixe de
compteur ajouté par PaperStream (`DOS-000456_0001.pdf`) est accepté.

Variante si « Other App » n'est pas utilisé : destination **Folder** vers
`C:\MULCV\numerisation\1-a-envoyer`, et installer avec la surveillance :
`powershell -ExecutionPolicy Bypass -File C:\MULCV\install.ps1 -Poste 1 -Surveillance`.

## D'où ça part, où ça arrive

1. PaperStream crée `C:\MULCV\numerisation\1-a-envoyer\DOS-000456.pdf` et lance `bin\mulcv-depot.exe`.
2. Le programme s'identifie auprès de Keycloak avec le compte du poste (`mulcv-depot-poste1` ou `2`).
3. Il envoie le PDF à MULCV avec l'identifiant `DOS-000456`.
4. MULCV le range dans **`Scan / Production / <zone> / <carton> / DOS-000456`**, le dossier créé
   quand GeoArchiv a transféré le lot. L'OCR l'analyse, puis l'archiviste le valide.
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

```
cd C:\MULCV
git pull
```
La configuration (`config\mulcv-depot.json`) et les fichiers de `numerisation\` ne sont pas
touchés par `git pull`.

## Administration et développement

- `admin\keycloak\` : définitions des comptes des postes. Ils sont créés sur le serveur par
  `provision_intake_clients.py` (dépôt `MULCV-search-service`) et doivent être
  **Collaborateurs** du site MULCV dans Share.
- `./build.sh` : tests et reconstruction de `bin\mulcv-depot.exe` (Go 1.24.7). La CI refuse un
  exécutable qui ne correspond pas exactement aux sources de `src\`.
- Contrat de l'API : `docs/transfer-api.md` du dépôt `MULCV-search-service`.
