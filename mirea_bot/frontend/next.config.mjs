/** @type {import('next').NextConfig} */
const nextConfig = {
  eslint: {
    ignoreDuringBuilds: true,
  },
  typescript: {
    ignoreBuildErrors: true,
  },
  images: {
    unoptimized: true,
  },
  // Возвращаем статический экспорт для сборки в папку out
  output: 'export',
  // В dev используем .next (watcher его не трогает). В production build — out (статический экспорт)
  distDir: process.env.NODE_ENV === 'production' ? 'out' : '.next',
  env: {
    API_BASE_URL: process.env.API_BASE_URL,
    NEXT_PUBLIC_API_BASE_URL: process.env.API_BASE_URL,
    TELEGRAM_BOT_NAME: process.env.TELEGRAM_BOT_NAME,
    // Кнопка «Тестовый вход»: подставляется при сборке, для Docker передать при build
    NEXT_PUBLIC_ENABLE_DEV_AUTH: process.env.NEXT_PUBLIC_ENABLE_DEV_AUTH,
  },
  // Настройки для статического экспорта
  trailingSlash: true,
  skipTrailingSlashRedirect: true,
}

export default nextConfig
