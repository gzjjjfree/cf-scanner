export namespace progressbar {
	
	export class Theme {
	    Saucer: string;
	    AltSaucerHead: string;
	    SaucerHead: string;
	    SaucerPadding: string;
	    BarStart: string;
	    BarEnd: string;
	    BarStartFilled: string;
	    BarEndFilled: string;
	
	    static createFrom(source: any = {}) {
	        return new Theme(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Saucer = source["Saucer"];
	        this.AltSaucerHead = source["AltSaucerHead"];
	        this.SaucerHead = source["SaucerHead"];
	        this.SaucerPadding = source["SaucerPadding"];
	        this.BarStart = source["BarStart"];
	        this.BarEnd = source["BarEnd"];
	        this.BarStartFilled = source["BarStartFilled"];
	        this.BarEndFilled = source["BarEndFilled"];
	    }
	}

}

export namespace wails {
	
	export class WSMessage {
	    type: string;
	    data: any;
	
	    static createFrom(source: any = {}) {
	        return new WSMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.data = source["data"];
	    }
	}
	export class WailsLogger {
	    Theme: progressbar.Theme;
	    Ctx: any;
	
	    static createFrom(source: any = {}) {
	        return new WailsLogger(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Theme = this.convertValues(source["Theme"], progressbar.Theme);
	        this.Ctx = source["Ctx"];
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

}

