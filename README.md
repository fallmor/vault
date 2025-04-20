# GitLab Vault Integration

Un programme en Go qui effectue les tâches suivantes :
- Utilise un fichier de configuration et des arguments CLI pour obtenir des informations de l'utilisateur.
- Se connecte à un serveur Vault avec AppRole ou un token Vault.
- Récupère un token GitLab depuis le serveur Vault.
- Se connecte à GitLab et liste les projets dans un groupe GitLab.
- Ajoute un fichier `README.md` et un fichier `gitlab-ci.yml` aux projets.
- Ajoute et met à jour des variables de projet.

## Utilisation

1. Configurez le fichier de configuration ou utilisez des arguments CLI pour fournir les informations nécessaires.
2. Lancez le programme :
   ```bash
   go mod tidy
   go run main.go 
   ```
3. Listez les arguments possibles.
    ```bash
    go run main --help




