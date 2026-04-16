export function parseRoomDetails(roomQuery: string): { campus: string, floor: number, roomId: string } | null {
    if (!roomQuery) return null;

    // Убираем лишние пробелы и переводим в верхний регистр для единообразия
    const cleanQuery = roomQuery.trim().toUpperCase();

    // СУПЕР-ВСЕЯДНАЯ РЕГУЛЯРКА:
    // 1. Ищет буквы (А-Я, A-Z) - это корпус
    // 2. Игнорирует любые тире, точки или пробелы между буквами и цифрами
    // 3. Ищет первую цифру (базовый этаж)
    // 4. Ищет остальные 2 или 3 цифры кабинета
    const match = cleanQuery.match(/([А-ЯЁA-Z]+)[-\s.]*(\d)(\d{2,3})/);

    if (!match) return null;

    const rawCampus = match[1];
    const firstDigit = parseInt(match[2], 10);
    
    // Формируем чистый roomId так, как он записан в SVG (обычно Буква-Цифры)
    // Если в запросе не было тире, мы его добавим для надежности поиска
    const roomId = cleanQuery.includes('-') ? cleanQuery : `${rawCampus}-${match[2]}${match[3]}`;

    let campus = "v-78"; 
    let floor = firstDigit; // Базовое предположение

    // Определяем корпус
    if (rawCampus.includes("С") || rawCampus.includes("C")) {
        campus = "s-20";
    } else if (rawCampus.includes("МП") || rawCampus.includes("MP")) {
        campus = "mp-1";
    } else if (rawCampus.includes("В") || rawCampus.includes("V") || rawCampus.includes("Г") || rawCampus.includes("G")) {
        campus = "v-78"; // Корпус Г физически находится в В-78
    }

    return { campus, floor, roomId };
}
