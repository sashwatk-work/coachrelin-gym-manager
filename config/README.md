# Container configuration

Configuration that is baked into or mounted into a container lives under the
directory named for that container:

- `frontend/` contains the static React Nginx configuration.
- `nginx/` contains the public TLS reverse-proxy template.

Environment files and Compose definitions remain under `docker/`.
