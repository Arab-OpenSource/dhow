# ADR-0012: مزوّدو Git عند الإطلاق

- **الحالة:** مقبول (مرحلة التصميم)
- **التاريخ:** 2026-10-27

## السياق
المستخدمين متوزعين على مزوّدين مختلفين، ومستخدمو الاستضافة الذاتية غالبًا بيشغّلوا Gitea أو Forgejo.

## القرار
ندعم GitHub (App)، وGitLab (SaaS والـ self-managed)، وBitbucket (Cloud وData Center)، وGitea/Forgejo/أي Git عام. الـ webhooks بتستخدم HMAC secret لكل مستودع، مع حماية من إعادة التشغيل (replay) بالـ delivery-id.

## البدائل اللي اتدرست
نبدأ بـ GitHub بس.

## العواقب
طبقة تجريد للمزوّدين (provider abstraction) من اليوم الأول. تقارير الحالة والفحوصات (status وchecks) خاصة بكل مزوّد.
