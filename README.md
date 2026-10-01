# Backend Golang — Belajar DevOps EKS

Repo backend (terpisah dari frontend). REST API in-memory, dideploy ke namespace `belajar-devops` di cluster EKS yang sama dengan frontend.

Repo: `git@gitlab.com:devops-training621338/backend.git`

## Struktur

```
belajar-devops-backend/
├── main.go           # REST API: GET/POST /api/items, /health, /api/info
├── go.mod
├── Dockerfile        # multi-stage -> distroless (~2MB)
├── k8s/
│   ├── deployment.yaml   # namespace belajar-devops (diasumsikan sudah ada)
│   ├── service.yaml      # ClusterIP :8080 (internal, nggak ke internet)
│   └── kustomization.yaml
└── .gitlab-ci.yml    # build -> ECR (belajar-backend) -> deploy EKS
```

## Hubungan dengan frontend (polyrepo)

- **Frontend** (`to-eks` repo) punya namespace + configmap + ALB. Diakses dari internet.
- **Backend** (repo ini) cuma deployment + service `ClusterIP`. Diakses frontend lewat DNS internal:
  `http://backend.belajar-devops.svc.cluster.local:8080`
- Browser **tidak** akses backend langsung — frontend yang jadi proxy.

## Urutan deploy (PENTING)

Namespace `belajar-devops` dibuat oleh repo frontend. Jadi:
1. Deploy **frontend** dulu (bikin namespace + configmap).
2. Baru deploy **backend** (repo ini).

Kalau backend di-deploy duluan sebelum namespace ada, pipeline gagal (`namespaces "belajar-devops" not found`).

## Endpoint

- `GET  /api/items` — daftar item
- `POST /api/items` — tambah item (`{"text":"..."}`)
- `GET  /health` — probe
- `GET  /api/info` — info pod

## Catatan: data in-memory

Data disimpan di memori — hilang kalau pod restart, dan tiap replica punya data sendiri. Fase berikutnya: tambah storage (PVC) biar persist.

## Push pertama

```powershell
cd belajar-devops-backend
git init
git branch -M main
git remote add origin git@gitlab.com:devops-training621338/backend.git
git add .
git commit -m "initial: backend golang + k8s + ci"
git push -u origin main
```

Jangan lupa set CI variable `AWS_ACCESS_KEY_ID` & `AWS_SECRET_ACCESS_KEY` di project GitLab backend, dan bikin ECR repo `belajar-backend`.
