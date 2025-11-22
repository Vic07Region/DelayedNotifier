const API_BASE = 'http://localhost:8080/notify'; // замените при необходимости

function displayResult(elId, content, isError = false) {
    const el = document.getElementById(elId);
    el.textContent = content;
    el.className = isError ? 'error' : 'success';
}

function getCurrentTimeString() {
    return new Date().toISOString().replace('T', ' ').substring(0, 19) + ' (получено сейчас)';
}

async function callApi(method, url, body = null) {
    const options = {
        method,
        headers: { 'Content-Type': 'application/json' }
    };
    if (body) options.body = JSON.stringify(body);
    const res = await fetch(url, options);
    const data = await res.json();
    if (!res.ok) {
        throw new Error(data.error || `HTTP ${res.status}`);
    }
    return data;
}

// Вспомогательные функции валидации
function isValidEmail(str) {
    const re = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    return re.test(str);
}

function isValidTelegram(str) {
    // Должно начинаться с @, затем 5–32 символа: буквы, цифры, подчёркивания
    const re = /^@[a-zA-Z0-9_]{5,32}$/;
    return re.test(str);
}

// Обновление метки и валидации при смене канала
const channelSelect = document.getElementById('channel');
const recipientInput = document.getElementById('recipient');
const recipientLabel = document.getElementById('recipient-label');
const recipientError = document.getElementById('recipient-error');

function updateRecipientValidation() {
    const channel = channelSelect.value;
    if (channel === 'email') {
        recipientLabel.querySelector('span')?.remove();
        recipientInput.placeholder = 'yourmail@email.ru';
        recipientInput.type = 'text'; // остаётся text, но валидируем как email
    } else if (channel === 'telegram') {
        recipientInput.placeholder = '@your_username';
    }
    validateRecipient();
}

function getTimezoneHourOffset() {
    const offsetMinutes = new Date().getTimezoneOffset(); // в минутах
    const offsetHours = -offsetMinutes / 60; // переворачиваем знак
    if (offsetHours > 0 && offsetHours < 10) {
        return '+0'+ String(offsetHours)
    }
    if (offsetHours >= 10) {
        return '+'+ String(offsetHours)
    }
    if (offsetHours < 0 && offsetHours > -9) {
        return '-0'+ String(offsetHours)
    }
    if (offsetHours < -10) {
        return '-'+ String(offsetHours)
    }
}

function validateRecipient() {
    const channel = channelSelect.value;
    const value = recipientInput.value.trim();
    recipientError.textContent = '';

    if (!value) return;

    let isValid = false;
    if (channel === 'email') {
        isValid = isValidEmail(value);
        if (!isValid) recipientError.textContent = 'Некорректный email';
    } else if (channel === 'telegram') {
        isValid = isValidTelegram(value);
        if (!isValid) recipientError.textContent = 'Telegram: должен начинаться с @, 5–32 символа (латиница, цифры, _)';
    }

    recipientInput.style.borderColor = isValid ? '#ddd' : '#e74c3c';
    return isValid;
}

channelSelect.addEventListener('change', updateRecipientValidation);
recipientInput.addEventListener('input', validateRecipient);

// Payload поля
const payloadFields = document.getElementById('payload-fields');

document.getElementById('add-payload-field').addEventListener('click', () => {
    const div = document.createElement('div');
    div.className = 'payload-field';
    const keyInput = document.createElement('input');
    keyInput.type = 'text';
    keyInput.placeholder = 'Ключ';
    keyInput.required = true;
    const valueInput = document.createElement('input');
    valueInput.type = 'text';
    valueInput.placeholder = 'Значение';
    valueInput.required = true;
    const removeBtn = document.createElement('button');
    removeBtn.textContent = '🗑️';
    removeBtn.type = 'button';
    removeBtn.addEventListener('click', () => div.remove());
    div.append(keyInput, valueInput, removeBtn);
    payloadFields.appendChild(div);
});

// Инициализация
window.addEventListener('DOMContentLoaded', () => {
    updateRecipientValidation();
    // Добавим 2 поля по умолчанию
    document.getElementById('add-payload-field').click();
    document.getElementById('add-payload-field').click();
    const timezoneValue = document.getElementById('time_zone');
    // Установим время по умолчанию — через 10 минут
    const now = new Date();
    timezoneValue.innerHTML = etTimezoneHourOffset();
// Добавляем 100 дней
//     now.setDate(now.getDate() + 100);

// Добавляем 10 минут
    now.setMinutes(now.getMinutes() + 10);

// Форматируем локальное время для <input type="datetime-local">
    const year = now.getFullYear();
    const month = String(now.getMonth() + 1).padStart(2, '0'); // месяцы 0-11
    const day = String(now.getDate()).padStart(2, '0');
    const hours = String(now.getHours()).padStart(2, '0');
    const minutes = String(now.getMinutes()).padStart(2, '0');

    document.getElementById('scheduled_at').value = `${year}-${month}-${day}T${hours}:${minutes}`;
});

// Конвертация datetime-local +05:00 в ISO с TZ
function formatScheduledTime(localValue) {
    if (!localValue) throw new Error('Укажите время отправки');
    // localValue: "2025-11-22T15:30"
    const dt = new Date(localValue + ':00'); // добавляем секунды
    // Форматируем в ISO с +05:00
    const year = dt.getFullYear();
    const month = String(dt.getMonth() + 1).padStart(2, '0');
    const day = String(dt.getDate()).padStart(2, '0');
    const hours = String(dt.getHours()).padStart(2, '0');
    const minutes = String(dt.getMinutes()).padStart(2, '0');
    const seconds = String(dt.getSeconds()).padStart(2, '0');
    const timeoffset = getTimezoneHourOffset()
    return `${year}-${month}-${day}T${hours}:${minutes}:${seconds}${timeoffset}:00`;
}

// Отправка
document.getElementById('create-form').addEventListener('submit', async (e) => {
    e.preventDefault();

    // Валидация получателя
    if (!validateRecipient()) {
        recipientError.textContent = channelSelect.value === 'email'
            ? 'Проверьте email'
            : 'Проверьте Telegram @username';
        return;
    }

    const recipient = recipientInput.value.trim();
    const channel = channelSelect.value;
    const localTime = document.getElementById('scheduled_at').value;
    let scheduled_at;
    try {
        scheduled_at = formatScheduledTime(localTime);
    } catch (err) {
        displayResult('create-result', 'Укажите корректное время отправки', true);
        return;
    }

    // Сборка payload
    const payload = {};
    const fields = payloadFields.querySelectorAll('.payload-field');
    let valid = true;
    fields.forEach(field => {
        const key = field.children[0].value.trim();
        const value = field.children[1].value.trim();
        if (key && value) {
            payload[key] = value;
        } else if (key || value) {
            valid = false;
        }
    });

    if (!valid) {
        displayResult('create-result', 'Все поля payload должны быть заполнены полностью или удалены.', true);
        return;
    }

    if (Object.keys(payload).length === 0) {
        displayResult('create-result', 'Добавьте хотя бы одно поле в payload.', true);
        return;
    }

    try {
        const resp = await callApi('POST', API_BASE, {
            recipient,
            channel,
            payload: JSON.stringify(payload),
            scheduled_at
        });
        resp.result.received_at = getCurrentTimeString();
        displayResult('create-result', JSON.stringify(resp.result, null, 2));
    } catch (err) {
        displayResult('create-result', `Ошибка: ${err.message}`, true);
    }
});

// Получение / удаление (без изменений)
document.getElementById('get-btn').addEventListener('click', async () => {
    const id = document.getElementById('notification-id').value.trim();
    if (!id) {
        displayResult('manage-result', 'Укажите ID уведомления', true);
        return;
    }

    try {
        const resp = await callApi('GET', `${API_BASE}/${id}`);
        resp.result.received_at = getCurrentTimeString();
        displayResult('manage-result', JSON.stringify(resp.result, null, 2));
    } catch (err) {
        displayResult('manage-result', `Ошибка: ${err.message}`, true);
    }
});

document.getElementById('delete-btn').addEventListener('click', async () => {
    const id = document.getElementById('notification-id').value.trim();
    if (!id) {
        displayResult('manage-result', 'Укажите ID уведомления', true);
        return;
    }

    try {
        await callApi('DELETE', `${API_BASE}/${id}`);
        displayResult('manage-result', `Уведомление ${id} успешно отменено.`);
    } catch (err) {
        displayResult('manage-result', `Ошибка: ${err.message}`, true);
    }
});