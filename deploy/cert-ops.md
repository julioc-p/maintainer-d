# Certificate Maintenance for maintainer-d

maintainer-d uses a manually issued Let’s Encrypt certificate stored in the `maintainerd-tls` secret.

Repeat these steps before the cert expires (every ~60–90 days):

1. **Request a new certificate via certbot**
   ```bash
   sudo certbot certonly \
     --manual \
     --preferred-challenges dns \
     --key-type rsa \
     -d github-events.cncf.io \
     --email <EMAIL_ADDRESS> \
     --agree-tos
   ```
   Certbot prints a `_acme-challenge.github-events.cncf.io` TXT record. Ask the DNSimple admin to create it (TTL 60). Press Enter once DNS propagates.

2. **Load the cert into Kubernetes**
   ```bash
   kubectl create secret tls maintainerd-tls \
     --cert=/etc/letsencrypt/live/github-events.cncf.io/fullchain.pem \
     --key=/etc/letsencrypt/live/github-events.cncf.io/privkey.pem \
     -n maintainerd \
     --dry-run=client -o yaml | kubectl apply -f -
   ```

3. **Recreate the Service so OCI reloads the cert**
   ```bash
   kubectl delete svc maintainerd -n maintainerd
   kubectl apply -f deploy/manifests/service.yaml
   kubectl get svc maintainerd -n maintainerd --watch
   ```
   Wait until `EXTERNAL-IP` shows `170.9.21.206` again.

4. **Verify**
   ```bash
   kubectl describe svc maintainerd -n maintainerd
   curl -vk https://github-events.cncf.io/healthz
   openssl s_client -connect github-events.cncf.io:443 -servername github-events.cncf.io -tls1_2
   ```

Keep `/etc/letsencrypt` backed up or document the certbot host. If you ever automate DNS updates, you can replace the manual step with cert-manager and remove the monthly coordination with DNSimple.

## HTTP-01 cert-manager flow (current)

We use cert-manager with HTTP-01 challenges and the nginx ingress controller. DNS for the public hosts must point at the nginx ingress controller LoadBalancer for issuance and renewal.

Prerequisites:
- `github-events.cncf.io` A record points to the nginx ingress controller external IP.
- IngressClass `nginx` exists and is reachable from the public internet.

Apply these manifests (checked into `deploy/manifests`):

```bash
kubectl apply -f deploy/manifests/clusterissuer-letsencrypt-http.yaml
kubectl apply -f deploy/manifests/certificate-github-events.yaml
kubectl apply -f deploy/manifests/ingress-github-events.yaml
```

Watch for completion:
```bash
kubectl -n maintainerd get certificate,order,challenge
kubectl -n maintainerd describe certificate maintainerd-tls
```

When `maintainer-d.cncf.io` is ready, apply:
```bash
kubectl apply -f deploy/manifests/ingress-maintainerd-web.yaml
```

## Cutover checklist (adding maintainer-d.cncf.io)

1. Confirm DNS:
   ```bash
   dig +short maintainer-d.cncf.io
   ```
   Expected: nginx ingress LoadBalancer IP.

2. Apply the ingress:
   ```bash
   kubectl apply -f deploy/manifests/ingress-maintainerd-web.yaml
   ```

3. Update the certificate to include the new host:
   ```bash
   kubectl apply -f deploy/manifests/certificate-github-events.yaml
   ```

4. If a new Order/Challenge does not appear within a minute, force a reissue:
   ```bash
   kubectl -n maintainerd delete challenge,order \
     -l acme.cert-manager.io/order-name=maintainerd-tls-1-3601081319
   ```

5. Watch issuance:
   ```bash
   kubectl -n maintainerd get certificate,order,challenge -w
   ```

6. Verify TLS:
   ```bash
   curl -vk https://maintainer-d.cncf.io/
   openssl s_client -connect maintainer-d.cncf.io:443 -servername maintainer-d.cncf.io -tls1_2
   ```

7. Verify web and BFF routing:
   ```bash
   curl -vk https://maintainer-d.cncf.io/
   curl -vk https://maintainer-d.cncf.io/api/healthz
   ```
