# A common spec, for the bite of CheckAddresses

## C-5. Маршруты

| Адрес | Метод | Доступ | Ответ | Вью-модель |
|---|---|---|---|---|
| `/login` | GET, POST | — | страница | `LoginPage` |
| `/logout` | POST | вход | редирект | — |
| `/setup` | GET, POST | — | страница | `SetupPage` |
| `/about` | GET | — | страница | `AboutPage` |
| `/app/{kind}/{account}/{col}` | GET | вход | страница | `ContactsListPage` |
| `/app/{kind}/{account}/{col}/{uid}` | GET | вход | страница | `ContactPage` |
| `/app/publish` | GET, POST | вход | лист | `PublishSheet` |

## C-6. Вью-модели
