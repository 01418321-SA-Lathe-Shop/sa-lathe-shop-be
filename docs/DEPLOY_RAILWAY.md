# Deploy backend บน Railway (ไม่ต้องใช้ Docker บนเครื่อง)

เอกสารนี้สำหรับนักศึกษาในทีมที่จะ deploy เพื่อพัฒนา ทดสอบ และส่งงาน ไม่ใช่ production setup Railway build `Dockerfile` บน server ของ Railway เอง เครื่องเราจึงไม่ต้องติดตั้ง Docker หรือ Go

ภาพรวม: 1 project บน Railway มี 3 อย่าง คือ **Postgres**, service **api** และ service **worker** ส่วนไฟล์ private ใช้ storage ที่รองรับ S3 API (ดูขั้นที่ 2)

> ค่าใช้จ่าย: Railway trial เป็นเครดิตจำกัดที่หมดอายุ (ราว 30 วันตามข้อมูลล่าสุดที่ทีมทราบ ให้ตรวจหน้า pricing ของ Railway อีกครั้ง) ดูยอดใช้จริงใน **Usage** ของ project เป็นระยะ ถ้าไม่ใช้นานๆ สามารถ scale service เป็น 0 หรือลบ project ได้

## 1. สร้าง project และฐานข้อมูล

1. railway.com → **New Project** → **Deploy from GitHub repo** → เลือก repo backend นี้ (ถ้ายังไม่เคยเชื่อม ให้อนุญาต Railway GitHub App ก่อน)
2. Railway จะสร้าง service แรกให้ ตั้งชื่อว่า `api` (ตั้งค่าในขั้นที่ 3)
3. ใน project กด **+ New** → **Database** → **PostgreSQL** (ชื่อ service เริ่มต้นคือ `Postgres`)

## 2. เตรียม storage แบบ S3 (สำหรับไฟล์แบบ/หลักฐาน/รูป)

backend ต้องการ bucket ที่รองรับ S3 API และ presigned URL ให้เลือกอย่างใดอย่างหนึ่ง:

- **Railway Bucket** (ถ้ามีในแพ็กเกจของคุณ: **+ New** → **Bucket**) แล้วจดค่า endpoint, bucket name, region, access key, secret key จากแท็บ credentials
- **Cloudflare R2** (มี free tier): สร้าง bucket แบบ private → สร้าง **R2 API token** สิทธิ์ Object Read & Write → endpoint คือ `https://<ACCOUNT_ID>.r2.cloudflarestorage.com`, region `auto`

ตั้ง **CORS ของ bucket** ให้เบราว์เซอร์อัปโหลด/ดาวน์โหลดตรงได้ (ใส่โดเมน Vercel ภายหลังได้):

```json
[
  {
    "AllowedOrigins": ["http://localhost:3000", "https://<ชื่อโปรเจกต์>.vercel.app"],
    "AllowedMethods": ["GET", "PUT"],
    "AllowedHeaders": ["Content-Type", "Content-Length"],
    "ExposeHeaders": ["ETag"],
    "MaxAgeSeconds": 300
  }
]
```

ห้ามเปิด bucket เป็น public ไฟล์ทั้งหมดใช้ signed link อายุ 10 นาทีเท่านั้น

## 3. ตั้งค่า service `api`

**Settings**
- **Config-as-code / Railway config file**: `railway.api.json` (กำหนด `Dockerfile.api`, health check `/health/ready` และ pre-deploy ที่รัน migration อัตโนมัติก่อน deploy ทุกครั้ง)
- **Source branch**: `develop` (หรือ branch ที่ทีมใช้ส่งงาน)
- **Networking** → **Generate Domain** → port `8080` แล้วจด URL เช่น `https://api-xxxx.up.railway.app`

**Variables** (แท็บ Variables → Raw Editor):

```env
DATABASE_URL=${{Postgres.DATABASE_URL}}
HTTP_ADDR=:8080
PORT=8080
CORS_ORIGINS=http://localhost:3000,https://<ชื่อโปรเจกต์>.vercel.app
AUTH_SESSION_TTL=12h
S3_ENDPOINT=<endpoint จากขั้นที่ 2>
S3_PUBLIC_ENDPOINT=<endpoint เดียวกัน>
S3_REGION=<region เช่น auto>
S3_BUCKET=<ชื่อ bucket>
S3_ACCESS_KEY=<access key>
S3_SECRET_KEY=<secret key>
S3_PATH_STYLE=true
LINE_ENABLED=false
```

- `CORS_ORIGINS` ต้องเป็น origin ล้วน (ไม่มี `/` ท้าย, ไม่มี path) คั่นด้วย comma
- ค่า secret ใส่ใน Railway Variables เท่านั้น ห้าม commit ลง repo

กด **Deploy** แล้วดู log: pre-deploy จะรัน `migrate up` (000001–000012) ก่อนเริ่ม API ถ้า migration ล้มเหลว deploy นั้นจะไม่ถูกเปิดใช้

ตรวจว่าใช้ได้: เปิด `https://<api-domain>/health/ready` ต้องได้ JSON `status` ok

## 4. ตั้งค่า service `worker`

1. **+ New** → **GitHub Repo** → repo backend เดิมอีกครั้ง → ตั้งชื่อ `worker`
2. **Config-as-code**: `railway.worker.json` (ใช้ `Dockerfile.worker`) และ branch เดียวกับ `api`
3. **Variables**: คัดลอกชุดเดียวกับ `api` ได้เลย (อย่างน้อย `DATABASE_URL`, ค่า `S3_*`, `LINE_ENABLED=false`) worker ไม่ต้องมี domain
4. worker ใช้ส่งการแจ้งเตือนในระบบ (outbox → inbox) และยกเลิกคำขอที่เลยกำหนด Deposit/PO 14 วัน ถ้าปิด worker ไว้ flow หลักยังใช้ได้ แต่การแจ้งเตือนจะไม่เข้า inbox

## 5. สร้าง Admin คนแรก (ทำครั้งเดียว)

ติดตั้ง Railway CLI บนเครื่อง (`npm i -g @railway/cli`) → `railway login` → `railway link` (เลือก project) แล้วเข้า shell ของ service api:

```sh
railway ssh --service api
# ภายใน container:
BOOTSTRAP_ADMIN_USERNAME=admin BOOTSTRAP_ADMIN_NAME="Shop Admin" BOOTSTRAP_ADMIN_PASSWORD='<รหัสผ่านอย่างน้อย 12 ตัวอักษร>' /app/api -bootstrap-admin
exit
```

คำสั่งนี้สร้าง Admin แล้วจบทันที ไม่กระทบ API ที่รันอยู่ จากนั้นใช้ Admin สร้างบัญชี Staff ในหน้า "จัดการพนักงาน" และให้ลูกค้าสมัครเองที่ `/register`

## 6. เชื่อมกับ frontend

ตั้ง `NEXT_PUBLIC_API_BASE_URL=https://<api-domain>` ใน Vercel (ดูคู่มือ frontend `docs/12-deploy-vercel-railway.md`) แล้วเพิ่มโดเมน Vercel ใน `CORS_ORIGINS` และ CORS ของ bucket

## แก้ปัญหาที่พบบ่อย

| อาการ | สาเหตุที่น่าจะเป็น |
| --- | --- |
| หน้าเว็บขึ้น "Cannot reach the API…" | `NEXT_PUBLIC_API_BASE_URL` ผิด, ยังไม่ redeploy Vercel หลังแก้ env, หรือ `CORS_ORIGINS` ไม่มีโดเมน Vercel |
| อัปโหลดไฟล์ไม่ผ่าน / "Upload again" | CORS ของ bucket ไม่มีโดเมน หรือ `S3_*` ผิด |
| deploy ค้างที่ health check | migration ล้มเหลว (ดู log pre-deploy) หรือ `DATABASE_URL` ไม่ได้อ้าง `Postgres` |
| `/health/ready` ได้ 503 | API ต่อฐานข้อมูลไม่ได้ |

ข้อจำกัด: ยังไม่ได้ทดสอบ image นี้บน Railway จริง (ตอนเขียนเอกสาร เครื่องผู้เขียนไม่มี Docker/Go) ถ้า build ล้มเหลวให้ดู log ขั้น `go install` ของ `migrate`
