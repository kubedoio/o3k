# Verifying O3K Releases

Every tagged release is signed and ships with a software bill of
materials (SBOM). Operators are expected to verify both before
deploying. This page is the recipe.

> **Trust model.** O3K uses [Sigstore](https://www.sigstore.dev/)
> keyless signing. There is no long-lived release key to compromise.
> Each artifact is signed by the GitHub Actions OIDC identity
> `https://github.com/senolcolak/o3kio/.github/workflows/release.yml@refs/tags/vX.Y.Z`.
> The signature certificate is logged to the public Rekor transparency
> log. If a signature verifies and the identity matches, the artifact
> came from this repository's release workflow at the named tag.

## What ships with each release

For every `vX.Y.Z` tag:

| Artifact | Purpose |
|---|---|
| `o3k-{linux,darwin}-{amd64,arm64}` | Pre-built binaries (`compat-check-*` ditto) |
| `checksums.txt` | SHA-256 manifest covering every published file |
| `checksums.txt.sig` + `checksums.txt.pem` | cosign signature + certificate over the manifest |
| `o3k-X.Y.Z.spdx.json` | SPDX 2.3 SBOM — Go modules and transitive deps |
| `o3k-X.Y.Z.spdx.json.sig` + `.pem` | cosign signature + certificate over the SBOM |
| `ghcr.io/senolcolak/o3kio:X.Y.Z` (image) | Multi-arch container image, signed in registry |

Container images additionally carry an SPDX SBOM as a Sigstore
attestation, retrievable with `cosign verify-attestation`.

## Prerequisites

Install [`cosign`](https://docs.sigstore.dev/cosign/installation/) v2.x:

```bash
# macOS
brew install cosign

# Linux
curl -LO https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64
sudo install cosign-linux-amd64 /usr/local/bin/cosign
```

## Verifying a binary release

The signature is over `checksums.txt`. Verify the manifest, then the
binaries against the manifest. Anyone tampering with a binary would
have to break SHA-256 *and* forge a Sigstore signature anchored to
this repo's GitHub Actions identity.

```bash
TAG=v0.7.2
REPO=senolcolak/o3kio

# 1. Fetch the artifacts you care about + the signed manifest.
gh release download "$TAG" -R "$REPO" \
  -p 'o3k-linux-amd64' \
  -p 'checksums.txt' -p 'checksums.txt.sig' -p 'checksums.txt.pem'

# 2. Verify the cosign signature on checksums.txt.
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature   checksums.txt.sig \
  --certificate-identity-regexp "https://github.com/${REPO}/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer     "https://token.actions.githubusercontent.com" \
  checksums.txt
# → "Verified OK" on success.

# 3. Confirm the binary's SHA-256 matches the verified manifest.
sha256sum -c --ignore-missing checksums.txt
# → "o3k-linux-amd64: OK"
```

If either step fails, **do not run the binary**. Open an issue with
the tag and SHA-256 you observed.

## Verifying the SBOM

Same flow, but against the SBOM:

```bash
TAG=v0.7.2
VERSION=${TAG#v}
REPO=senolcolak/o3kio

gh release download "$TAG" -R "$REPO" \
  -p "o3k-${VERSION}.spdx.json" \
  -p "o3k-${VERSION}.spdx.json.sig" \
  -p "o3k-${VERSION}.spdx.json.pem"

cosign verify-blob \
  --certificate "o3k-${VERSION}.spdx.json.pem" \
  --signature   "o3k-${VERSION}.spdx.json.sig" \
  --certificate-identity-regexp "https://github.com/${REPO}/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer     "https://token.actions.githubusercontent.com" \
  "o3k-${VERSION}.spdx.json"
```

The SBOM lists every Go module compiled into the binaries with its
exact version. Pipe into your scanner of choice:

```bash
# Using grype (Anchore)
grype sbom:o3k-${VERSION}.spdx.json

# Using Trivy
trivy sbom o3k-${VERSION}.spdx.json
```

## Verifying the container image

Container signatures live in the OCI registry next to the image, not
as separate files. Use `cosign verify` directly:

```bash
TAG=v0.7.2
IMAGE=ghcr.io/senolcolak/o3kio:${TAG#v}
REPO=senolcolak/o3kio

cosign verify "$IMAGE" \
  --certificate-identity-regexp "https://github.com/${REPO}/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer     "https://token.actions.githubusercontent.com"
```

Pull the SBOM attestation:

```bash
cosign verify-attestation "$IMAGE" \
  --type spdxjson \
  --certificate-identity-regexp "https://github.com/${REPO}/.github/workflows/release.yml@refs/tags/v.*" \
  --certificate-oidc-issuer     "https://token.actions.githubusercontent.com" \
  | jq -r '.payload' | base64 -d | jq '.predicate' > image.spdx.json
```

`image.spdx.json` is now the verified SBOM for the running image,
which you can scan or archive alongside your deployment record.

## Admission control

If you run on Kubernetes, [policy-controller](https://docs.sigstore.dev/policy-controller/overview/)
or Kyverno can enforce signature verification cluster-wide. Minimal
Kyverno policy:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-o3k-signed
spec:
  validationFailureAction: Enforce
  rules:
    - name: verify-o3k
      match:
        any:
          - resources:
              kinds: [Pod]
      verifyImages:
        - imageReferences:
            - "ghcr.io/senolcolak/o3kio:*"
          attestors:
            - entries:
                - keyless:
                    subjectRegExp: "https://github.com/senolcolak/o3kio/.github/workflows/release.yml@refs/tags/v.*"
                    issuer: "https://token.actions.githubusercontent.com"
```

## What we do NOT yet do

Honest scope:

- **No SLSA Level 3 provenance** — the signing identity is
  workflow-anchored, but we do not currently emit a separate
  `attestation` payload describing the build inputs. Track the
  upstream [SLSA generator](https://github.com/slsa-framework/slsa-github-generator)
  if you need full provenance.
- **No reproducible builds** — binaries are stripped (`-w -s`) but
  Go embeds module paths and timestamps. Two tags built independently
  will not bytewise match.
- **No artifact mirror** — releases live only on GitHub. If GitHub is
  unavailable, verification is impossible. Mirror artifacts you
  depend on into your own storage.
- **No vulnerability gating at release time** — `govulncheck` runs in
  CI on every push but the release workflow does not re-run it. Treat
  the SBOM as the source of truth and scan it yourself.

## Reporting a verification failure

If `cosign verify` fails on a published artifact:

1. Capture the full command and output.
2. Note the tag and the SHA-256 of the artifact you have locally.
3. File a security advisory via GitHub's `Security` tab —
   **do not open a public issue**. See
   [SECURITY.md](../SECURITY.md).
