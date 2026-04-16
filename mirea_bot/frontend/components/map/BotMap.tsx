"use client"

import React, { useEffect, useRef, useState, useMemo } from "react"
import { TransformWrapper, TransformComponent, ReactZoomPanPinchRef } from "react-zoom-pan-pinch"
import { Plus, Minus, ChevronLeft, MapPin } from "lucide-react"

// --- СТАТИЧЕСКИЕ ИМПОРТЫ SVG ---
import v78_0 from "@/components/svg-maps/v-78/floor_0.svg"
import v78_1 from "@/components/svg-maps/v-78/floor_1.svg"
import v78_2 from "@/components/svg-maps/v-78/floor_2.svg"
import v78_3 from "@/components/svg-maps/v-78/floor_3.svg"
import v78_4 from "@/components/svg-maps/v-78/floor_4.svg"
import s20_1 from "@/components/svg-maps/s-20/floor_1.svg"
import s20_2 from "@/components/svg-maps/s-20/floor_2.svg"
import s20_3 from "@/components/svg-maps/s-20/floor_3.svg"
import s20_4 from "@/components/svg-maps/s-20/floor_4.svg"
import mp1_m1 from "@/components/svg-maps/mp-1/-1.svg"
import mp1_1 from "@/components/svg-maps/mp-1/1.svg"
import mp1_2 from "@/components/svg-maps/mp-1/2.svg"
import mp1_3 from "@/components/svg-maps/mp-1/3.svg"
import mp1_4 from "@/components/svg-maps/mp-1/4.svg"
import mp1_5 from "@/components/svg-maps/mp-1/5.svg"

const svgRegistry: Record<string, any> = {
    "v-78_0": v78_0, "v-78_1": v78_1, "v-78_2": v78_2, "v-78_3": v78_3, "v-78_4": v78_4,
    "s-20_1": s20_1, "s-20_2": s20_2, "s-20_3": s20_3, "s-20_4": s20_4,
    "mp-1_-1": mp1_m1, "mp-1_1": mp1_1, "mp-1_2": mp1_2, "mp-1_3": mp1_3, "mp-1_4": mp1_4, "mp-1_5": mp1_5,
};

const campuses = [
  { id: 'v-78', shortName: 'В-78', floors: [{ level: 0, label: '0' }, { level: 1, label: '1' }, { level: 2, label: '2' }, { level: 3, label: '3' }, { level: 4, label: '4' }]},
  { id: 's-20', shortName: 'С-20', floors: [{ level: 1, label: '1' }, { level: 2, label: '2' }, { level: 3, label: '3' }, { level: 4, label: '4' }]},
  { id: 'mp-1', shortName: 'МП-1', floors: [{ level: -1, label: '-1' }, { level: 1, label: '1' }, { level: 2, label: '2' }, { level: 3, label: '3' }, { level: 4, label: '4' }, { level: 5, label: '5' }]}
];

interface BotMapProps {
  onClose: () => void;
}

export function BotMap({ onClose }: BotMapProps) {
    const [currentCampus, setCampus] = useState('v-78');
    const [currentFloor, setFloor] = useState(1);
    
    const [svgText, setSvgText] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState(false);
    
    const [hasCenteredInitial, setHasCenteredInitial] = useState(false);
    
    const transformRef = useRef<ReactZoomPanPinchRef>(null);

    const svgUrl = useMemo(() => {
        const item = svgRegistry[`${currentCampus}_${currentFloor}`];
        return typeof item === 'string' ? item : item?.src || null;
    }, [currentCampus, currentFloor]);

    useEffect(() => {
        if (!svgUrl) return;
        setIsLoading(true);
        fetch(svgUrl, { cache: 'force-cache' })
            .then(res => res.text())
            .then(text => {
                setSvgText(text.replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, ""));
                setIsLoading(false);
            })
            .catch(() => setIsLoading(false));
    }, [svgUrl]);

    // Сильное отдаление при первом открытии
    useEffect(() => {
        if (!isLoading && svgText && !hasCenteredInitial) {
            const timer = setTimeout(() => {
                // Масштаб 0.15, чтобы покрыть огромные SVG на экране телефона
                transformRef.current?.centerView(0.15, 0);
                setHasCenteredInitial(true);
            }, 50);
            return () => clearTimeout(timer);
        }
    }, [isLoading, svgText, hasCenteredInitial]);

    const campusData = campuses.find(c => c.id === currentCampus) || campuses[0];

    return (
        <div className="fixed inset-0 z-[100] bg-background flex flex-col overflow-hidden animate-in fade-in duration-300">
            <div className="absolute top-0 left-0 right-0 p-4 z-[110] flex justify-between items-center pointer-events-none">
                <button 
                    onClick={onClose} 
                    className="pointer-events-auto bg-background/90 backdrop-blur-md border shadow-xl w-12 h-12 flex items-center justify-center rounded-full text-foreground active:scale-90 transition-all"
                >
                    <ChevronLeft className="w-6 h-6" />
                </button>
            </div>

            <div className="absolute right-4 top-[40%] -translate-y-1/2 z-50 flex flex-col gap-2.5">
                {campusData.floors.slice().reverse().map(f => (
                    <button 
                        key={f.level} 
                        onClick={() => setFloor(f.level)} 
                        className={`w-12 h-12 rounded-full shadow-lg font-black text-sm transition-all active:scale-90 flex items-center justify-center border-2 ${
                            currentFloor === f.level 
                                ? 'bg-primary text-primary-foreground border-primary scale-110' 
                                : 'bg-background/95 backdrop-blur-md text-foreground border-border hover:border-primary/50'
                        }`}
                        style={{ width: '48px', height: '48px' }}
                    >
                        {f.label}
                    </button>
                ))}
            </div>

            <div className="absolute bottom-28 left-4 z-50 pointer-events-auto">
                <div className="relative">
                    <select 
                        value={currentCampus} 
                        onChange={e => { setCampus(e.target.value); setFloor(1); }} 
                        className="appearance-none bg-background/95 backdrop-blur-md border-2 border-border shadow-lg pl-11 pr-6 py-3.5 rounded-2xl font-black text-sm outline-none focus:border-primary min-w-[140px] transition-all active:scale-95 text-left"
                    >
                        {campuses.map(c => <option key={c.id} value={c.id}>{c.shortName}</option>)}
                    </select>
                    <MapPin className="absolute left-4 top-1/2 -translate-y-1/2 w-5 h-5 text-primary pointer-events-none" />
                </div>
            </div>

            <div className="absolute bottom-28 right-4 z-50 flex flex-col gap-4 pointer-events-auto">
                <div className="bg-background/95 backdrop-blur-md border-2 border-border shadow-lg rounded-2xl flex flex-col p-1">
                    <button 
                        onClick={() => transformRef.current?.zoomIn()} 
                        className="w-11 h-11 flex items-center justify-center hover:bg-muted active:bg-accent rounded-xl text-primary transition-all active:scale-90"
                    >
                        <Plus className="w-6 h-6" />
                    </button>
                    <div className="h-px bg-border mx-2" />
                    <button 
                        onClick={() => transformRef.current?.zoomOut()} 
                        className="w-11 h-11 flex items-center justify-center hover:bg-muted active:bg-accent rounded-xl text-primary transition-all active:scale-90"
                    >
                        <Minus className="w-6 h-6" />
                    </button>
                </div>
            </div>

            <div className="flex-1 w-full h-full bg-[#f8fafc] dark:bg-[#020617] relative">
                <TransformWrapper 
                    ref={transformRef} 
                    initialScale={0.15} 
                    minScale={0.02} 
                    maxScale={10}
                >
                    <TransformComponent wrapperStyle={{ width: '100%', height: '100%' }}>
                        <div id="svg-map-container" className={`w-full h-full flex items-center justify-center p-6 transition-opacity duration-500 ${isLoading ? 'opacity-30' : 'opacity-100'}`}>
                            {svgText ? (
                                <div 
                                    dangerouslySetInnerHTML={{ __html: svgText }} 
                                    className="w-full h-full flex items-center justify-center [&>svg]:max-w-none [&>svg]:max-h-none [&>svg]:w-auto [&>svg]:h-auto transition-all duration-300" 
                                />
                            ) : (
                                <div className="flex flex-col items-center gap-4">
                                    <div className="w-12 h-12 border-4 border-primary border-t-transparent rounded-full animate-spin" />
                                </div>
                            )}
                        </div>
                    </TransformComponent>
                </TransformWrapper>
            </div>
        </div>
    )
}
