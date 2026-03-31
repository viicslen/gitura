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
	export class CommentDTO {
	    id: number;
	    in_reply_to_id: number;
	    body: string;
	    author_login: string;
	    author_avatar: string;
	    diff_hunk: string;
	    created_at: string;
	    is_suggestion: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CommentDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.in_reply_to_id = source["in_reply_to_id"];
	        this.body = source["body"];
	        this.author_login = source["author_login"];
	        this.author_avatar = source["author_avatar"];
	        this.diff_hunk = source["diff_hunk"];
	        this.created_at = source["created_at"];
	        this.is_suggestion = source["is_suggestion"];
	    }
	}
	export class CommentThreadDTO {
	    root_id: number;
	    node_id: string;
	    comments: CommentDTO[];
	    resolved: boolean;
	    path: string;
	    line: number;
	
	    static createFrom(source: any = {}) {
	        return new CommentThreadDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root_id = source["root_id"];
	        this.node_id = source["node_id"];
	        this.comments = this.convertValues(source["comments"], CommentDTO);
	        this.resolved = source["resolved"];
	        this.path = source["path"];
	        this.line = source["line"];
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
	export class IgnoredCommenterDTO {
	    login: string;
	    // Go type: time
	    added_at: any;
	
	    static createFrom(source: any = {}) {
	        return new IgnoredCommenterDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.login = source["login"];
	        this.added_at = this.convertValues(source["added_at"], null);
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
	export class PullRequestSummary {
	    id: number;
	    number: number;
	    title: string;
	    state: string;
	    is_draft: boolean;
	    body: string;
	    head_branch: string;
	    base_branch: string;
	    head_sha: string;
	    node_id: string;
	    html_url: string;
	    owner: string;
	    repo: string;
	    comment_count: number;
	    unresolved_count: number;
	
	    static createFrom(source: any = {}) {
	        return new PullRequestSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.number = source["number"];
	        this.title = source["title"];
	        this.state = source["state"];
	        this.is_draft = source["is_draft"];
	        this.body = source["body"];
	        this.head_branch = source["head_branch"];
	        this.base_branch = source["base_branch"];
	        this.head_sha = source["head_sha"];
	        this.node_id = source["node_id"];
	        this.html_url = source["html_url"];
	        this.owner = source["owner"];
	        this.repo = source["repo"];
	        this.comment_count = source["comment_count"];
	        this.unresolved_count = source["unresolved_count"];
	    }
	}
	export class SuggestionCommitResult {
	    commit_sha: string;
	    html_url: string;
	
	    static createFrom(source: any = {}) {
	        return new SuggestionCommitResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.commit_sha = source["commit_sha"];
	        this.html_url = source["html_url"];
	    }
	}

}

