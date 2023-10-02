import {H3, LinkableContext} from "../components/header";
import React, {ReactElement, ReactNode, useContext, useState} from "react";

export type Language = "go" | "typescript"

const LangContext = React.createContext<{ lang: Language, setLang: (l: Language) => void }>({
    lang: "go",
    setLang: (lang) => {
    }
});

export const LanguageProvider = ({children}) => {
    const [lang, setLang] = useState<Language>("go")
    const context = {
        lang,
        setLang
    }

    return (
        <LangContext.Provider value={context}>
            {children}
        </LangContext.Provider>
    )
}

export const LanguageSelect = () => {
    const langContext = useContext(LangContext);
    return (
        <button style={{background: "tomato", margin: "24px", padding: "10px"}}
                onClick={() => langContext.setLang(langContext.lang === "go" ? "typescript" : "go")}>
            Language:
            {langContext.lang}
        </button>
    )
}

export const LanguageSwitch = (props: { langToContent: Record<Language, ReactNode> }): ReactElement => {
    const lang = useContext(LangContext).lang;
    return <LinkableContext.Provider value={false}>{props.langToContent[lang]}</LinkableContext.Provider>
}

export const LanguageOperation = (props: {
    usage: ReactNode,
    parameters: ReactNode,
    response: ReactNode
}) => {
    return <>
        <div style={{display: "flex", width: "1200px"}}>
            <div style={{flex: 1}}>
                <H3>Parameters</H3>
                {props.parameters}
                <H3>Response</H3>
                {props.response}
            </div>
            <div style={{flex: 1}}>
                {props.usage}
            </div>
        </div>
    </>
}