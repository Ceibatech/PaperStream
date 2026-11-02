# PaperStream

Dépôt automatique des documents numérisés avec **PaperStream Capture** dans **MULCV**.

## Installer un poste (5 minutes)

1. **Cloner** le dépôt sur le poste, par exemple dans `C:\MULCV\PaperStream` :
   ```
   git clone https://github.com/Ceibatech/PaperStream.git C:\MULCV\PaperStream
   ```
2. **Récupérer le secret du poste** auprès de l'administrateur. Il le lit sur le serveur avec
   `cat /opt/ceiba/keycloak/secrets/mulcv-depot-poste1.secret` (ou `poste2`).
3. **Double-cliquer sur `installer.cmd`**, taper le numéro du poste (`1` ou `2`), puis coller le secret.
   La fenêtre doit finir par « Connexion à MULCV vérifiée ».

Rien d'autre à installer : `bin/mulcv-depot.exe` est déjà compilé, et la CI vérifie à chaque
modification qu'il correspond exactement aux sources de `src/`.

Pour essayer sans scanner : **`tester-envoi.cmd`**, glisser un PDF dans la fenêtre, puis taper
l'identifiant d'un dossier déjà transféré (par exemple `TEST-DOS-001`).

## Paramétrer PaperStream Capture

Dans le profil de numérisation :

| Réglage | Valeur |
|---|---|
| Séparation de documents | **Code-barres**, type **QR Code** (le QR du dossier) |
| Nom du fichier (*File Name Option*) | **valeur du code-barres** seule, sans préfixe, date ni compteur |
| Format | PDF ou PDF/A, 25 Mo au plus par dossier |
| Destination | **Other App** |
| Application Path | `C:\MULCV\mulcv-depot.exe` |
| Parameters | `%f` |
| Work Folder | `C:\MULCV` |

Le nom du fichier **doit** être l'identifiant du dossier : `DOS-000456.pdf`. Un nom de
compteur (`0001.pdf`) ne permet pas de savoir où ranger le document ; il est refusé et
mis de côté. Un suffixe de compteur PaperStream (`DOS-000456_0001.pdf`) est accepté.

Variante sans « Other App » : destination **Folder** vers `C:\MULCV\Sortie`, et installation
avec `powershell -ExecutionPolicy Bypass -File install.ps1 -Poste 1 -Surveillance`.

## Ce qui se passe à chaque numérisation

1. PaperStream crée `DOS-000456.pdf` et lance `mulcv-depot.exe` avec ce fichier.
2. Le programme obtient un jeton Keycloak avec le compte du poste (`mulcv-depot-poste1` ou `2`).
3. Il envoie le PDF à MULCV avec l'identifiant `DOS-000456`.
4. MULCV le range dans `Scan / Production / <zone> / <carton> / DOS-000456`
   (dossier créé lors du transfert GeoArchiv) ; l'OCR prend ensuite le relais.
5. Sur le poste :

| Résultat | Emplacement |
|---|---|
| Déposé dans MULCV | `C:\MULCV\Envoyes` |
| Refusé (dossier inconnu, format…) | `C:\MULCV\Erreurs`, avec un `.txt` qui donne la raison |
| Réseau ou MULCV indisponible | reste en place ; 4 essais, puis nouvel essai au prochain envoi |

Journal : `C:\MULCV\mulcv-depot.log`. Vérifier la connexion à tout moment : `C:\MULCV\mulcv-depot.exe -test`.

## Les deux postes

| Poste | Compte technique | Configuration |
|---|---|---|
| Poste 1 | `mulcv-depot-poste1` | `postes/poste-1.json` |
| Poste 2 | `mulcv-depot-poste2` | `postes/poste-2.json` |

Chaque poste a son propre compte : MULCV sait quel poste a déposé chaque document, et un
poste perdu se révoque sans arrêter l'autre. Les comptes sont créés sur le serveur par
`provision_intake_clients.py` (dépôt `MULCV-search-service`), et doivent être
**Collaborateurs** du site MULCV dans Share.

## Mettre à jour un poste

```
cd C:\MULCV\PaperStream
git pull
installer.cmd        (garder la configuration existante : répondre O)
```

## Développement

```
./build.sh            # tests + reconstruction de bin/mulcv-depot.exe (Go 1.24.7)
```
Commiter `bin/mulcv-depot.exe` avec les sources : la CI refuse un exécutable qui ne correspond pas.

Contrat de l'API : `docs/transfer-api.md` du dépôt `MULCV-search-service`.
