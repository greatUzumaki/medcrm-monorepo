#!/usr/bin/env bash
# Interactive: asks for the mailbox password and the Telegram bot token, writes /etc/leads.env (0600).
# Run on the server as root:  ssh -t root@IP leads-setup
set -euo pipefail

read -rp "Почтовый ящик для заявок (логин SMTP) [hello@openmedai.ru]: " SMTP_USER
SMTP_USER=${SMTP_USER:-hello@openmedai.ru}
read -rsp "Пароль от ящика $SMTP_USER (не отображается): " SMTP_PASS; echo
read -rp "Куда слать заявки [$SMTP_USER]: " MAIL_TO
MAIL_TO=${MAIL_TO:-$SMTP_USER}

read -rsp "Токен Telegram-бота от @BotFather (Enter — пропустить): " TG_BOT_TOKEN; echo
TG_CHAT_ID=""
if [ -n "$TG_BOT_TOKEN" ]; then
  echo "Напиши боту любое сообщение в Telegram (или добавь его в группу и напиши там), затем нажми Enter."
  read -r
  # getUpdates returns the latest chat that wrote to the bot.
  TG_CHAT_ID=$(curl -fsS "https://api.telegram.org/bot${TG_BOT_TOKEN}/getUpdates" \
    | grep -o '"chat":{"id":-\?[0-9]*' | tail -1 | grep -o -- '-\?[0-9]*$' || true)
  if [ -z "$TG_CHAT_ID" ]; then
    read -rp "Не нашёл чат автоматически. Введи chat_id вручную: " TG_CHAT_ID
  else
    echo "chat_id: $TG_CHAT_ID"
  fi
fi

umask 077
cat > /etc/leads.env <<EOF
SMTP_USER=$SMTP_USER
SMTP_PASS=$SMTP_PASS
MAIL_TO=$MAIL_TO
TG_BOT_TOKEN=$TG_BOT_TOKEN
TG_CHAT_ID=$TG_CHAT_ID
EOF
chmod 600 /etc/leads.env
systemctl restart leads
sleep 1
systemctl is-active leads && echo "Готово. Отправь тестовую заявку с сайта."
