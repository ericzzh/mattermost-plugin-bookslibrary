
import {DispatchFunc, GenericAction, GetStateFunc} from 'mattermost-redux/types/actions';
import {GlobalState} from 'mattermost-redux/types/store';
import {UserTimezone,UserProfile} from 'mattermost-redux/types/users';
import moment from 'moment-timezone';
import {
    searchPostsWithParams,
    searchFilesWithParams,
} from 'mattermost-redux/actions/search';

//copy from mattermost-webapp/actions/post_action
export function searchForTerm(term:string) {

    return (dispatch:DispatchFunc) => {
        dispatch(updateSearchTerms(term));
        dispatch(showSearchResults());
        return {data: true};
    };
}

export function updateSearchTerms(terms: string) {
    return {
        type: "UPDATE_RHS_SEARCH_TERMS",
        terms,
    };
}

export function showSearchResults(isMentionSearch = false) {
    return (dispatch: DispatchFunc, getState: GetStateFunc) => {
        const state = getState() as GlobalState;

        const searchTerms = getSearchTerms(state);

        if (isMentionSearch) {
            dispatch(updateRhsState('mention'));
        } else {
            dispatch(updateRhsState('search'));
        }
        dispatch(updateSearchResultsTerms(searchTerms));

        return dispatch(performSearch(searchTerms));
    };
}

export function updateRhsState(rhsState: string, channelId?: string) {
    return (dispatch: DispatchFunc, getState: GetStateFunc) => {
        const action = {
            type: 'UPDATE_RHS_STATE',
            state: rhsState,
        } as GenericAction;

        if (rhsState === 'pin' || rhsState === 'channel-files') {
            action.channelId = channelId || getCurrentChannelId(getState());
        }

        dispatch(action);

        return {data: true};
    };
}

function updateSearchResultsTerms(terms: string) {
    return {
        type: 'UPDATE_RHS_SEARCH_RESULTS_TERMS',
        terms,
    };
}

export function performSearch(terms: string, isMentionSearch?: boolean) {
    return (dispatch: DispatchFunc, getState: GetStateFunc) => {
        const teamId = getCurrentTeamId(getState());
        const config = getConfig(getState());
        const viewArchivedChannels = config.ExperimentalViewArchivedChannels === 'true';
        const extensionsFilters = getFilesSearchExtFilter(getState() as GlobalState);

        const extensions = extensionsFilters?.map((ext) => `ext:${ext}`).join(' ');
        let termsWithExtensionsFilters = terms;
        if (extensions?.trim().length > 0) {
            termsWithExtensionsFilters += ` ${extensions}`;
        }

        // timezone offset in seconds
        const userId = getCurrentUserId(getState());
        const userTimezone = getUserTimezone(getState(), userId);
        const userCurrentTimezone = getUserCurrentTimezone(userTimezone);
        const timezoneOffset = ((userCurrentTimezone && (userCurrentTimezone.length > 0)) ? getUtcOffsetForTimeZone(userCurrentTimezone) : getBrowserUtcOffset()) * 60;
        const messagesPromise = dispatch(searchPostsWithParams(teamId, {terms, is_or_search: Boolean(isMentionSearch), include_deleted_channels: viewArchivedChannels, time_zone_offset: timezoneOffset, page: 0, per_page: 20}));
        const filesPromise = dispatch(searchFilesWithParams(teamId, {terms: termsWithExtensionsFilters, is_or_search: Boolean(isMentionSearch), include_deleted_channels: viewArchivedChannels, time_zone_offset: timezoneOffset, page: 0, per_page: 20}));
        return Promise.all([filesPromise, messagesPromise]);
    };
}

export function getSearchTerms(state: any): string {
    return state.views.rhs.searchTerms;
}
export function getCurrentTeamId(state: GlobalState) {
    return state.entities.teams.currentTeamId;
}
export function getConfig(state: GlobalState): any {
    return state.entities.general.config;
}
export function getFilesSearchExtFilter(state:any ): string[] {
    return state.views.rhs.filesSearchExtFilter;
}
export function getCurrentUserId(state: GlobalState): string {
    return state.entities.users.currentUserId;
}
export function getUserTimezone(state: GlobalState, id: string) {
    const profile = state.entities.users.profiles[id];
    return getTimezoneForUserProfile(profile);
}
export function getUserCurrentTimezone(userTimezone?: UserTimezone): string | undefined | null {
    if (!userTimezone) {
        return null;
    }
    const {
        useAutomaticTimezone,
        automaticTimezone,
        manualTimezone,
    } = userTimezone;

    let useAutomatic = useAutomaticTimezone;
    if (typeof useAutomaticTimezone === 'string') {
        useAutomatic = useAutomaticTimezone === 'true';
    }

    if (useAutomatic) {
        return automaticTimezone;
    }
    return manualTimezone;
}
export function getBrowserUtcOffset() {
    return moment().utcOffset();
}
export function getCurrentChannelId(state: GlobalState): string {
    return state.entities.channels.currentChannelId;
}

export function getUtcOffsetForTimeZone(timezone:any) {
    return moment.tz(timezone).utcOffset();
}

export function getTimezoneForUserProfile(profile: UserProfile) {
    if (profile && profile.timezone) {
        return {
            ...profile.timezone,
            useAutomaticTimezone: profile.timezone.useAutomaticTimezone === 'true',
        };
    }

    return {
        useAutomaticTimezone: true,
        automaticTimezone: '',
        manualTimezone: '',
    };
}
