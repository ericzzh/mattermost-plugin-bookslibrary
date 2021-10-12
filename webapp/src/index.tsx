import { Store, Action } from "redux";

import { GlobalState } from "mattermost-redux/types/store";

import manifest from "./manifest";

// eslint-disable-next-line import/no-unresolved
import { PluginRegistry } from "./types/mattermost-webapp";

import BookType from "./components/BookType"
import BorrowType from "./components/BorrowType"
import reducer from "./ConfigSlice"

export default class Plugin {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/no-empty-function
    public async initialize(
        registry: PluginRegistry,
        store: Store<GlobalState, Action<Record<string, unknown>>>
    ) {
        // @see https://developers.mattermost.com/extend/plugins/webapp/reference/
        registry.registerPostTypeComponent("custom_book_type", BookType);
        registry.registerPostTypeComponent("custom_borrow_type", BorrowType);
        registry.registerReducer(reducer)
    }
}

declare global {
    interface Window {
        registerPlugin(id: string, plugin: Plugin): void;
        PostUtils: {
            formatText(arg:any):any
            messageHtmlToComponent(arg:any):any
          }
    }
}

window.registerPlugin(manifest.id, new Plugin());
