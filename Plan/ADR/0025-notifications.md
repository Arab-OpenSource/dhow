# ADR-0025: قنوات الإشعارات عند الإطلاق

- **الحالة:** مقبول (مرحلة التصميم)
- **التاريخ:** 2026-10-27

## السياق
الفرق بتتواصل عبر أدوات مختلفة، وWhatsApp مسيطر في مصر والخليج.

## القرار
البريد الإلكتروني، وwebhook عام موقَّع، وSlack، وMicrosoft Teams، وTelegram، وDiscord، وWhatsApp Business (Cloud API). التسليم بيستخدم queue مخصصة بمهلات (timeouts) وحارس SSRF. الموافقات التفاعلية في Slack وTeams ضمن Business+.

## البدائل اللي اتدرست
البريد + webhook بس.

## العواقب
محولات (adapters) أكتر للصيانة. WhatsApp محتاج حساب Meta Business بتاع العميل.
