# Pindah ke GitHub Actions (OIDC, tanpa access key)

Panduan pindah CI/CD dari GitLab ke GitHub Actions. Konsep sama (build → ECR → deploy EKS), beda sintaks + auth pakai OIDC.

## Beda utama dari GitLab CI

| GitLab | GitHub Actions |
|---|---|
| `.gitlab-ci.yml` | `.github/workflows/deploy.yml` |
| CI Variables (access key) | OIDC (nggak ada key disimpan) |
| `stages` + `script` | `jobs` + `steps` |

---

## LANGKAH 1 — Bikin 2 repo di GitHub

Lewat web GitHub, bikin 2 repo (private/public bebas):
- `belajar-devops-frontend`
- `belajar-devops-backend`

## LANGKAH 2 — Push kode ke GitHub

Repo backend (dari folder `belajar-devops-backend`):
```powershell
git remote add github https://github.com/<USERNAME>/belajar-devops-backend.git
git push github main
```
Repo frontend (dari folder `belajar-devops-eks`):
```powershell
git remote add github https://github.com/<USERNAME>/belajar-devops-frontend.git
git push github main
```
(Pakai remote nama `github` biar GitLab `origin` tetap ada. Atau ganti origin kalau mau total pindah.)

## LANGKAH 3 — Deploy CFN GitHub OIDC (infra)

Dari folder `belajar-devops-eks/cloudformation`:
```powershell
aws cloudformation deploy `
  --template-file 07-github-oidc-stack.yaml `
  --stack-name github-oidc `
  --parameter-overrides GitHubOrg=diofathur FrontendRepo=belajar-devops-frontend BackendRepo=belajar-devops-backend `
  --capabilities CAPABILITY_NAMED_IAM `
  --region us-east-1
```
Ambil ARN role dari output:
```powershell
aws cloudformation describe-stacks --stack-name github-oidc --region us-east-1 `
  --query "Stacks[0].Outputs[?OutputKey=='GitHubActionsRoleArn'].OutputValue" --output text
```
Simpan ARN itu (dipakai di workflow).

> Kalau OIDC provider GitHub udah pernah ada di akun, resource GitHubOidcProvider bakal
> bentrok. Hapus yang lama dulu atau buang resource itu dari stack & referensikan yang ada.

## LANGKAH 4 — Kasih role akses ke cluster (EKS Access Entry)

```powershell
aws eks create-access-entry --cluster-name eks-cluster `
  --principal-arn arn:aws:iam::183945808571:role/github-actions-ci --region us-east-1

aws eks associate-access-policy --cluster-name eks-cluster `
  --principal-arn arn:aws:iam::183945808571:role/github-actions-ci `
  --access-scope type=cluster `
  --policy-arn arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy `
  --region us-east-1
```

## LANGKAH 5 — Bikin workflow (LO yang ketik file ini)

Buat file `.github/workflows/deploy.yml` di repo backend. Ini kerangkanya + penjelasan.
Ketik sendiri biar paham tiap bagian:

```yaml
name: Deploy Backend to EKS

# Trigger: tiap push ke branch main
on:
  push:
    branches: [main]

# WAJIB buat OIDC: kasih izin workflow minta token id-token
permissions:
  id-token: write      # buat OIDC (assume role AWS)
  contents: read       # buat checkout kode

env:
  AWS_REGION: us-east-1
  ECR_REGISTRY: <ACCOUNT_ID>.dkr.ecr.us-east-1.amazonaws.com
  ECR_REPO: belajar-backend
  CLUSTER: eks-cluster

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest       # runner GitHub-hosted (padanan "image" di GitLab)
    steps:
      # 1. Ambil kode repo
      - name: Checkout
        uses: actions/checkout@v4

      # 2. Login ke AWS pakai OIDC (INI inti bedanya - nggak ada access key!)
      - name: Configure AWS credentials (OIDC)
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: <GitHubActionsRoleArn>   # dari output stack 07
          aws-region: ${{ env.AWS_REGION }}

      # 3. Login ke ECR
      - name: Login to ECR
        uses: aws-actions/amazon-ecr-login@v2

      # 4. Build + push image (tag = commit SHA)
      - name: Build and push
        run: |
          IMAGE=$ECR_REGISTRY/$ECR_REPO:${{ github.sha }}
          docker build -t $IMAGE -t $ECR_REGISTRY/$ECR_REPO:latest .
          docker push $IMAGE
          docker push $ECR_REGISTRY/$ECR_REPO:latest

      # 5. Setup kubectl
      - name: Setup kubectl
        uses: azure/setup-kubectl@v4

      # 6. Deploy ke EKS
      - name: Deploy
        run: |
          aws eks update-kubeconfig --name $CLUSTER --region $AWS_REGION
          IMAGE=$ECR_REGISTRY/$ECR_REPO:${{ github.sha }}
          kubectl kustomize k8s | sed "s|image: backend:local|image: $IMAGE|g" | kubectl apply -f -
          kubectl -n belajar-devops rollout status deployment/backend --timeout=180s
```

Yang perlu diganti di workflow:
- `<ACCOUNT_ID>` → account AWS lo
- `<GitHubActionsRoleArn>` → output stack 07
- Buat frontend: ganti `ECR_REPO`, path build (`./frontend`), nama deployment, path k8s.

## LANGKAH 6 — Push & lihat jalan

```powershell
git add .github/workflows/deploy.yml
git commit -m "ci: github actions deploy ke eks via oidc"
git push github main
```
Buka tab **Actions** di repo GitHub → lihat workflow jalan.

---

## Kenapa OIDC lebih baik dari access key (yang di GitLab)

- **Nggak ada secret disimpan**: GitLab nyimpen AWS_ACCESS_KEY_ID + SECRET. GitHub Actions OIDC nggak nyimpen apa-apa — token di-generate on-the-fly tiap run, cuma hidup beberapa menit.
- **Scoped ketat**: trust policy cuma izinin repo + branch tertentu (lihat Condition di stack 07). Bocor pun nggak kepakai dari tempat lain.
- **Konsep sama IRSA**: OIDC + role + trust. Lo udah paham ini dari EBS CSI / ALB controller.

## Catatan

Semua ini disiapkan buat dijalankan nanti (butuh GitHub repo + lab AWS hidup).
Workflow YAML sengaja NGGAK dibuatin filenya - lo yang ketik di `.github/workflows/deploy.yml`
biar paham strukturnya. Kerangka di atas tinggal disalin & disesuaikan.
