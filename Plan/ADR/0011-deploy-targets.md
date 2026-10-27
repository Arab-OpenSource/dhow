# ADR-0011: أهداف النشر (deploy targets) في v1.0

- **الحالة:** مقبول (مرحلة التصميم)
- **التاريخ:** 2026-10-27

## السياق
المستخدمين المستهدفين بيشغّلوا أساطيل VPS، وأجهزة Docker، وKubernetes، واستضافة ثابتة (static) أو مشتركة.

## القرار
v1.0 بيشحن: SSH/VPS (إصدارات بـ symlink ذرّي، وفحوصات صحة، وrollback تلقائي)، وأجهزة Docker/Compose، وKubernetes (Helm/Kustomize/manifests مع متابعة حالة الـ rollout)، وStatic/CDN/SFTP. مفاتيح الـ host وبصمات TLS بتتثبّت (pinned) لكل target.

## البدائل اللي اتدرست
نشحن SSH بس في الأول.

## العواقب
نطاق v1.0 أكبر، لكنه متوافق مع السوق. كل target محتاج اختبارات E2E تغطي الفشل والـ rollback.
