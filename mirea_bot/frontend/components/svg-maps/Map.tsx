import { useEffect, useState } from "react"
import Spinner from "@/components/ui/Spinner"

const Map = ({ svgUrl }: { svgUrl: string }) => {
  const [data, setData] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState(false)

  useEffect(() => {
    if (!svgUrl) return;
    
    setIsLoading(true);
    setError(false);

    // Делаем прямой запрос к публичной папке
    fetch(svgUrl)
      .then(async (res) => {
        if (!res.ok) throw new Error("SVG не найден");
        let text = await res.text();
        
        // Удаляем потенциально проблемные скрипты из SVG
        text = text.replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, "");
        
        setData(text);
        setIsLoading(false);
      })
      .catch((err) => {
        console.error("Ошибка при загрузке карты", err);
        setError(true);
        setIsLoading(false);
      });
  }, [svgUrl]);

  return (
    <div className="w-full h-full flex items-center justify-center relative">
      {isLoading && (
        <div className="absolute inset-0 flex items-center justify-center bg-background/50 z-50">
          <Spinner />
        </div>
      )}
      
      {error && (
        <div className="text-muted-foreground p-4 bg-background/80 rounded-lg">
          Не удалось загрузить схему этажа
        </div>
      )}
      
      {!isLoading && !error && data && (
        <div 
            dangerouslySetInnerHTML={{ __html: data }} 
            id="map" 
            className="w-full h-full flex items-center justify-center" 
        />
      )}
    </div>
  )
}

export default Map;
