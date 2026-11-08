// Company data in one place. Placeholder values — replace before launch.
export const site = {
  brand: 'MedCRM',
  phone: '+7 495 000-00-00',
  phoneHref: 'tel:+74950000000',
  email: 'hello@medcrm.ru',
  telegram: 'https://t.me/medcrm',
  telegramLabel: '@medcrm',
  // Lead endpoint of apps/api (POST /public/leads). Unset → form reports it is not connected.
  leadsEndpoint: import.meta.env.PUBLIC_LEADS_ENDPOINT as string | undefined,
};
