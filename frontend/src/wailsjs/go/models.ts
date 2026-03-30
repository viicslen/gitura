export namespace model {
	
	export class AuthState {
	    is_authenticated: boolean;
	    login: string;
	    avatar_url: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.is_authenticated = source["is_authenticated"];
	        this.login = source["login"];
	        this.avatar_url = source["avatar_url"];
	    }
	}
	export class DeviceFlowInfo {
	    device_code: string;
	    user_code: string;
	    verification_uri: string;
	    expires_in: number;
	    interval: number;
	
	    static createFrom(source: any = {}) {
	        return new DeviceFlowInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device_code = source["device_code"];
	        this.user_code = source["user_code"];
	        this.verification_uri = source["verification_uri"];
	        this.expires_in = source["expires_in"];
	        this.interval = source["interval"];
	    }
	}
	export class PRListFilters {
	    include_author: boolean;
	    include_assignee: boolean;
	    include_reviewer: boolean;
	    repo: string;
	    org: string;
	    author: string;
	    updated_after: string;
	    include_drafts: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PRListFilters(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.include_author = source["include_author"];
	        this.include_assignee = source["include_assignee"];
	        this.include_reviewer = source["include_reviewer"];
	        this.repo = source["repo"];
	        this.org = source["org"];
	        this.author = source["author"];
	        this.updated_after = source["updated_after"];
	        this.include_drafts = source["include_drafts"];
	    }
	}
	export class PRListItem {
	    number: number;
	    title: string;
	    owner: string;
	    repo: string;
	    author_login: string;
	    created_at: string;
	    updated_at: string;
	    html_url: string;
	    is_draft: boolean;
	    is_author: boolean;
	    is_assignee: boolean;
	    is_reviewer: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PRListItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.title = source["title"];
	        this.owner = source["owner"];
	        this.repo = source["repo"];
	        this.author_login = source["author_login"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.html_url = source["html_url"];
	        this.is_draft = source["is_draft"];
	        this.is_author = source["is_author"];
	        this.is_assignee = source["is_assignee"];
	        this.is_reviewer = source["is_reviewer"];
	    }
	}
	export class PRListResult {
	    items: PRListItem[];
	    rate_limit_reset?: string;
	    incomplete_results?: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PRListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], PRListItem);
	        this.rate_limit_reset = source["rate_limit_reset"];
	        this.incomplete_results = source["incomplete_results"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PollResult {
	    status: string;
	    error?: string;
	    interval?: number;
	
	    static createFrom(source: any = {}) {
	        return new PollResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.error = source["error"];
	        this.interval = source["interval"];
	    }
	}

}

