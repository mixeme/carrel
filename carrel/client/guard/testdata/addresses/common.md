# A common spec, for the bite of CheckAddresses

## C-5. Маршруты

| Адрес | Метод | Доступ | Ответ | Вью-модель |
|---|---|---|---|---|
| `/login` | GET, POST | — | страница | `LoginPage` |
| `/logout` | POST | вход | редирект | — |
| `/setup` | GET, POST | — | страница | `SetupPage` |
| `/invite/{token}` | GET, POST | токен | страница | `InvitePage`; мёртвый секрет — `DeadLinkPage` |
| `/about` | GET | — | страница | `AboutPage` |
| `/app/{kind}` | GET | вход | страница | те же, что строкой ниже |
| `/app/{kind}/{account}/{col}` | GET | вход | страница | `ContactsListPage`, `AgendaPage` |
| `/app/{kind}/{account}/{col}/{uid}` | GET | вход | страница | `ContactPage` |
| `/app/publish` | GET, POST | вход | лист | `PublishSheet` |
| `/admin/reset` | GET, POST | админ | лист | `PasswordResetSheet`; `?target=`; ответ — лист с `Issued` |
| `/app/nothing` | GET | вход | страница | — |

## C-6. Вью-модели
