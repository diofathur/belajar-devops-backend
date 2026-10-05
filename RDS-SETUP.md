# Setup RDS PostgreSQL + Secrets Manager (Cara A)

Panduan ganti backend dari file (PVC) ke RDS PostgreSQL, dengan password diambil dari AWS Secrets Manager (cara A: ambil via CLI → K8s Secret).

Cluster: `eks-cluster` | Account: `183945808571` | Region: `us-east-1`

---

## Alur

```
Stack 06 bikin RDS -> password auto ke Secrets Manager
   -> ambil password via CLI -> bikin K8s Secret
   -> backend baca DB_PASSWORD dari Secret, host/user dari ConfigMap
   -> backend connect ke RDS (PostgreSQL)
```

---

## LANGKAH 1 — Bikin RDS (pilih: Console ATAU CloudFormation)

### Opsi A — Lewat Console (lebih visual, cocok buat belajar)

1. Console → **RDS** → **Create database**
2. **Choose a database creation method**: Standard create
3. **Engine options**: PostgreSQL
4. **Templates**: Free tier (atau Dev/Test)
5. **Settings**:
   - DB instance identifier: `belajar-postgres`
   - Master username: `appuser`
   - **Credentials management**: pilih **Managed in AWS Secrets Manager** ← INI kuncinya.
     RDS auto-generate password + simpan ke Secrets Manager. (nggak usah ketik password)
6. **Instance configuration**: db.t3.micro (atau t4g.micro)
7. **Storage**: 20 GB gp3, matikan autoscaling (hemat)
8. **Connectivity**:
   - VPC: pilih VPC EKS lo (`eks-cluster-vpc` / vpc-0f90...)
   - DB subnet group: bikin/pilih yang berisi **private subnet**
   - Public access: **No** (DB internal aja)
   - VPC security group: bikin baru, nanti kita atur izin 5432 (lihat catatan)
   - Availability Zone: no preference
9. **Additional configuration**:
   - Initial database name: `belajardb` ← penting, biar DB-nya langsung ada
10. **Create database**. Tunggu ~5-10 menit sampai status **Available**.

**Setelah jadi — atur security group** biar worker node EKS bisa konek:
- Buka security group RDS → Inbound rules → Add rule:
  - Type: PostgreSQL (5432)
  - Source: CIDR VPC lo (`10.0.0.0/16`) atau security group node EKS

> Kenapa "Managed in Secrets Manager"? Ini yang bikin password otomatis ke Secrets Manager
> tanpa lo ketik manual. Persis tujuan belajar lo: RDS + Secrets Manager.

### Opsi B — Lewat CloudFormation (otomatis, 1 command)

```powershell
cd "i:\My Drive\kerja\belajar-devops-eks\cloudformation"

aws cloudformation deploy `
  --template-file 06-rds-stack.yaml `
  --stack-name eks-rds `
  --parameter-overrides VpcStackName=eks-vpc `
  --region us-east-1
```

Stack ini otomatis bikin semua (RDS private + Secrets Manager + security group).
Bedanya dari console: nggak ada klik, semua eksplisit di template. Hasil sama.

## LANGKAH 2 — Ambil info koneksi

```powershell
aws cloudformation describe-stacks --stack-name eks-rds --region us-east-1 `
  --query "Stacks[0].Outputs" --output table
```

Catat: `DBEndpoint`, `DBPort` (5432), `DBName` (belajardb), `DBSecretArn`.

## LANGKAH 3 — Ambil password dari Secrets Manager → bikin K8s Secret

Tujuan: ambil password RDS (yang disimpan RDS di Secrets Manager), terus taruh ke K8s Secret
biar backend bisa baca. Password nggak pernah ditulis ke file/git.

### Opsi A — Lewat Console (lihat password langsung)

1. Console → **Secrets Manager** → **Secrets**
2. Cari secret bernama `rds!db-xxxxxxxx` (atau nama DB lo). Klik.
3. Scroll ke **Secret value** → klik **Retrieve secret value**
4. Muncul JSON:
   ```json
   { "username": "appuser", "password": "aB3xYz..." }
   ```
5. **Copy nilai `password`-nya** (yang panjang).
6. Balik ke terminal, bikin K8s Secret pakai password itu (ganti `<PASTE_PASSWORD>`):
   ```powershell
   kubectl create secret generic backend-db-secret `
     --namespace belajar-devops `
     --from-literal=DB_PASSWORD="<PASTE_PASSWORD>"
   ```

Simpel: lo lihat password di console, copy, bikin secret. Nggak perlu ngerti JSON parsing.

### Opsi B — Lewat CLI (per langkah, biar paham)

```powershell
# 1. Cari nama/ARN secret RDS lo
aws secretsmanager list-secrets --region us-east-1 `
  --query "SecretList[?contains(Name,'rds')].[Name,ARN]" --output table
```
Catat ARN-nya (atau Name).

```powershell
# 2. Ambil isi secret (output: JSON {"username":...,"password":...})
aws secretsmanager get-secret-value `
  --secret-id <ARN-atau-Name-dari-langkah-1> `
  --region us-east-1 `
  --query SecretString --output text
```
Lo bakal lihat JSON-nya. Copy nilai `password` dari situ.

```powershell
# 3. Bikin K8s Secret pakai password itu
kubectl create secret generic backend-db-secret `
  --namespace belajar-devops `
  --from-literal=DB_PASSWORD="<PASTE_PASSWORD>"
```

### Opsi C — CLI otomatis (1 alur, kalau mau cepet)

Kalau pakai CloudFormation (output DBSecretArn tersedia):
```powershell
$ARN  = (aws cloudformation describe-stacks --stack-name eks-rds --region us-east-1 --query "Stacks[0].Outputs[?OutputKey=='DBSecretArn'].OutputValue" --output text)
$JSON = (aws secretsmanager get-secret-value --secret-id $ARN --region us-east-1 --query SecretString --output text)
$PASS = ($JSON | ConvertFrom-Json).password
kubectl create secret generic backend-db-secret --namespace belajar-devops --from-literal=DB_PASSWORD="$PASS"
```
`ConvertFrom-Json` = ubah teks JSON jadi objek, lalu `.password` ambil field-nya. Otomatis, nggak copy-paste.

### Verifikasi (semua opsi)

```powershell
kubectl get secret backend-db-secret -n belajar-devops
```
Harus muncul secret `backend-db-secret` dengan 1 data (DB_PASSWORD).

## LANGKAH 4 — Isi DB_HOST di configmap

Edit `belajar-devops-backend/k8s/configmap.yaml`, isi `DB_HOST` dengan `DBEndpoint` dari Langkah 2.

## LANGKAH 5 — Lepas PVC + ubah manifest (lihat perubahan di bawah)

Edit manifest backend (lo yang ketik). Lalu push → CI build + deploy.

---

## Perubahan file backend (lo yang edit)

### A. `k8s/configmap.yaml`
Ganti `DATA_FILE` jadi info koneksi DB non-sensitif:
```yaml
data:
  PORT: "8080"
  DB_HOST: "<DBEndpoint-dari-output>"
  DB_PORT: "5432"
  DB_NAME: "belajardb"
  DB_USER: "appuser"
```

### B. `k8s/deployment.yaml`
- HAPUS `volumeMounts:` (mount /data)
- HAPUS `volumes:` (persistentVolumeClaim)
- TAMBAH env DB_PASSWORD dari secret:
```yaml
          env:
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: backend-db-secret
                  key: DB_PASSWORD
```

### C. `k8s/kustomization.yaml`
Hapus `- pvc.yaml` dan `- storageclass.yaml` dari resources.

### D. Hapus file
`k8s/pvc.yaml` dan `k8s/storageclass.yaml`.

### E. `main.go`
Ganti dari file JSON ke PostgreSQL (kode lengkap diberikan terpisah).

### F. `go.mod` + `Dockerfile`
Tambah dependency `github.com/lib/pq`, Dockerfile pakai `go mod tidy`.

---

## Teardown RDS (kalau selesai)

```powershell
aws cloudformation delete-stack --stack-name eks-rds --region us-east-1
```
Catatan: `DeletionPolicy: Snapshot` -> bikin snapshot (ada biaya). Secret di Secrets Manager jadwal hapus 7 hari.
