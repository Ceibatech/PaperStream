# PaperStream

Dépôt automatique des documents numérisés avec **PaperStream Capture** dans **MULCV**.

Chaque dossier physique porte un QR code avec son identifiant GeoArchiv. PaperStream
sépare les dossiers sur ce QR, nomme chaque PDF avec la valeur lue, puis lance
`mulcv-depot.exe`. Le programme envoie le fichier à MULCV, qui le range dans le
dossier transféré portant cet identifiant :

```
MULCV / Documents / Scan / Production / <zone> / <carton> / <dossier>
```

Le service OCR du dossier Scan analyse ensuite le document, puis l'archiviste le valide.

## Les deux postes

| Poste | Compte technique Keycloak | Configuration |
|---|---|---|
| Poste 1 | `mulcv-depot-poste1` | `postes/poste-1.json` |
| Poste 2 | `mulcv-depot-poste2` | `postes/poste-2.json` |

Chaque poste a son propre compte : le journal de MULCV indique quel poste a déposé
chaque document, et un poste perdu se révoque sans arrêter l'autre.

## Préparation (une fois, administrateur)

1. **Keycloak**, realm `mulcv` → Clients → *Import client* : `keycloak/mulcv-depot-poste1.client.json`
   puis `poste2`. Noter le secret de chacun (onglet *Credentials*).
2. **Backend MULCV** : `TRANSFERS_SCANNER_CLIENTS=mulcv-depot-poste1,mulcv-depot-poste2`.
3. **Share** → site MULCV → *Membres du site* : ajouter `service-account-mulcv-depot-poste1`
   et `…-poste2` comme **Collaborateurs** (ils apparaissent après le premier `-test`).

## Installation sur chaque poste

1. Télécharger `mulcv-paperstream.zip` (onglet *Releases*, ou artefact de la dernière
   exécution de la CI) et le décompresser.
2. Ouvrir PowerShell **avec le compte Windows de l'opérateur du poste**, dans le dossier décompressé :
   ```powershell
   powershell -ExecutionPolicy Bypass -File install.ps1 -Poste 1   # ou -Poste 2
   ```
   Le script crée `C:\MULCV`, demande le secret du poste sans l'afficher, protège
   la configuration (lisible par l'opérateur et les administrateurs seulement)
   et teste la connexion.

## Profil PaperStream Capture

1. **Séparation de documents** : *Code-barres*, type **QR Code**.
2. **Nom du fichier** (*File Name Option*) : la **valeur du QR**, sans préfixe ni date.
   Un compteur `_0001` ajouté par PaperStream est accepté.
3. **Format** : PDF ou PDF/A, 25 Mo au plus par dossier.
4. **Destination** : **Other App**
   - Application Path : `C:\MULCV\mulcv-depot.exe`
   - Parameters : `%f`
   - Work Folder : `C:\MULCV`

Variante : destination **Folder** vers `C:\MULCV\Sortie` et installation avec
`-Surveillance` ; une tâche planifiée envoie alors les fichiers déposés dans ce dossier.

## Suivi

| Résultat | Emplacement |
|---|---|
| Déposé dans MULCV | `C:\MULCV\Envoyes` |
| Refusé (dossier inconnu, format…) | `C:\MULCV\Erreurs` + un `.txt` avec la raison |
| Réseau ou MULCV indisponible | reste en place ; 4 essais puis nouvel essai au prochain envoi |

Journal : `C:\MULCV\mulcv-depot.log`. Vérification à tout moment : `C:\MULCV\mulcv-depot.exe -test`.

## Développement

```
cd src
go test ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o mulcv-depot.exe .
```

Contrat de l'API : `docs/transfer-api.md` du dépôt `MULCV-search-service`.
