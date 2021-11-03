import React from "react";
import { useHistory } from "react-router-dom";
import PropTypes from "prop-types";
import Grid from "@material-ui/core/Grid";
import Paper from "@material-ui/core/Paper";
import TextField from "@material-ui/core/TextField";
import Chip from "@material-ui/core/Chip";
import AlarmOnIcon from "@material-ui/icons/AlarmOn";
import Button from "@material-ui/core/Button";
import IconButton from "@material-ui/core/IconButton";
import DeleteIcon from "@material-ui/icons/Delete";
import { StylesProvider } from "@material-ui/core/styles";
import styled from "@emotion/styled";
import { getCurrentTeam } from "mattermost-redux/selectors/entities/teams";
import { useSelector } from "react-redux";
import WorkerIcon from "@material-ui/icons/PermContactCalendar";
import BorrowIcon from "@material-ui/icons/MenuBook";
import HouseIcon from "@material-ui/icons/House";
import { useDispatch } from "react-redux";
import { GlobalState } from "mattermost-redux/types/store";
import moment from "moment";
import manifest from "../manifest";
import { getConfig } from "mattermost-redux/selectors/entities/general";
import Avatar from "@material-ui/core/Avatar";
// import Avatar from '@mui/material/Avatar';
import Stepper from "@material-ui/core/Stepper";
import Step from "@material-ui/core/Step";
import StepLabel from "@material-ui/core/StepLabel";
import StepConnector from "@material-ui/core/StepConnector";
import { StepIconProps } from "@material-ui/core/StepIcon";
import clsx from "clsx";
import useMediaQuery from "@material-ui/core/useMediaQuery";
import { useTheme } from "@material-ui/core/styles";
import Accordion from "@material-ui/core/Accordion";
import AccordionSummary from "@material-ui/core/AccordionSummary";
import AccordionDetails from "@material-ui/core/AccordionDetails";
import ExpandMoreIcon from "@material-ui/icons/ExpandMore";
import {
    fetchConfig,
    getExpireDays,
    getMaxRenewTimes,
    getStatus,
    getError,
} from "../ConfigSlice";
import { getCurrentUser } from "mattermost-redux/selectors/entities/common";
import { Client4 } from "mattermost-redux/client";
import InProgress from "./InProgress";
import MsgBox, { MsgBoxProps, useMessageUtils } from "./MsgBox";
import Image from "./Image";
import BookIcon from "@material-ui/icons/Book";
import { Template } from "../utils";

const STATUS_REQUESTED = "R";
const STATUS_CONFIRMED = "C";
const STATUS_DELIVIED = "D";
const STATUS_RENEW_REQUESTED = "RR";
const STATUS_RENEW_CONFIRMED = "RC";
const STATUS_RETURN_REQUESTED = "RTR";
const STATUS_RETURN_CONFIRMED = "RTC";
const STATUS_RETURNED = "RT";

const WORKFLOW_BORROW = "BORROW";
const WORKFLOW_RENEW = "RENEW";
const WORKFLOW_RETURN = "RETURN";

// const { formatText, messageHtmlToComponent } = window.PostUtils;

const TEXT: Record<string, string> = {
    ["WF_" + WORKFLOW_BORROW]: "借书流程",
    ["WF_" + WORKFLOW_RENEW]: "续借流程",
    ["WF_" + WORKFLOW_RETURN]: "还书流程",
    ["ST_" + STATUS_REQUESTED]: "借书请求",
    ["ST_" + STATUS_CONFIRMED]: "借书确认",
    ["ST_" + STATUS_DELIVIED]: "已收书",
    ["ST_" + STATUS_RENEW_REQUESTED]: "续借请求",
    ["ST_" + STATUS_RENEW_CONFIRMED]: "续借确认",
    ["ST_" + STATUS_RETURN_REQUESTED]: "还书请求",
    ["ST_" + STATUS_RETURN_CONFIRMED]: "还书确认",
    ["ST_" + STATUS_RETURNED]: "已还书",
    ["DATE_" + STATUS_REQUESTED]: "借阅请求日",
    ["DATE_" + STATUS_CONFIRMED]: "借阅确认日",
    ["DATE_" + STATUS_DELIVIED]: "收书日",
    ["DATE_" + STATUS_RENEW_REQUESTED]: "续借请求日",
    ["DATE_" + STATUS_RENEW_CONFIRMED]: "续借确认日",
    ["DATE_" + STATUS_RETURN_REQUESTED]: "还书请求日",
    ["DATE_" + STATUS_RETURN_CONFIRMED]: "还书确认日",
    ["DATE_" + STATUS_RETURNED]: "还书日",
    next_step: "下一步",
    date_accordion: "日期",
    CONFIRM_DELETE: "确定删除此借书请求吗？",
    ALERT_DELETE_ERROR: "删除失败,错误：",
    ALERT_DELETE_SUCC: "删除成功。",
    CONFIRM_STEP: `确定进入到状态：`,
    WORKFLOW_ERROR: "请求失败,错误：",
    WORKFLOW_SUCC: "请求成功",
    LOAD_CONFIG_ERROR: "加载配置数据失败。错误：",
    REJECT: "拒绝",
    DISTANCE_TO_RETRUN: "距还书还有%v天",
    DISTANCE_AFTER_RETRUN: "已超还书日%v天",
};

function FindStatusInWorkflow(status: string, workflow: Step[]) {
    return workflow.find((step) => step.status === status);
}

function BorrowType(props: any) {
    const post = { ...props.post };
    const message = post.message || "";

    //Fetching server config
    const config = useSelector(getConfig);

    //Current user
    const currentUser = useSelector(getCurrentUser);

    //Current Team
    const currentTeam = useSelector(getCurrentTeam);

    //history
    const history = useHistory();

    //Theme
    const defaultTheme = useTheme();
    const matchesSm = useMediaQuery(defaultTheme.breakpoints.up("sm"));

    //Fetching plugin config
    const expireDays = useSelector(getExpireDays);
    const maxRenewTimes = useSelector(getMaxRenewTimes);
    const loadStatus = useSelector(getStatus);
    const loadConfigError = useSelector(getError);

    //Loading
    const [loading, setLoading] = React.useState(false);

    //Message
    const mutil = useMessageUtils();

    //Effect
    React.useEffect(() => {
        if (loadConfigError) {
            mutil.setMsgBox({
                open: true,
                text: TEXT["LOAD_CONFIG_ERROR"] + loadConfigError,
                serverity: "error",
            });
        }
    }, [loadConfigError]);

    React.useEffect(() => {
        if (expireDays === -1 || maxRenewTimes === -1) {
            !loading && setLoading(true);
            if (loadStatus === "loading") {
                return;
            }
            dispatch(fetchConfig());
        } else {
            loading && setLoading(false);
        }
    }, [expireDays, maxRenewTimes]);

    //Dispatch
    const dispatch = useDispatch();

    //Async Message
    const postMessage = React.useRef(post.message);

    if (postMessage.current !== post.message) {
        mutil.checkAndDisplayAsyncMsg();
        postMessage.current = post.message;
    }

    //Image
    const imgSrc = React.useRef("");
    const [imgSrcReplace, setImgSrcReplace] = React.useState("");

    // const [imgSrc, setImgSrc] = React.useState(
    //     `${config.SiteURL}/plugins/${manifest.id}/public/info/${borrowReq.book_id}/cover.jpeg`
    // );
    //

    //Parsing message
    let borrow: Borrow;
    let borrowReq: BorrowRequest;
    let currentStep: Step;
    let workflow: Step[];

    try {
        borrow = JSON.parse(message);
        borrowReq = borrow.dataOrImage;
        workflow = borrow.dataOrImage.workflow;
        currentStep = borrowReq.workflow[borrowReq.step_index];
    } catch (error) {
        // const formattedText = messageHtmlToComponent(formatText(message));
        // return <div> {formattedText} </div>;
        return <div> {message} </div>;
    }

    imgSrc.current = `${config.SiteURL}/plugins/${manifest.id}/public/info/${borrowReq.book_id}/cover.jpeg`;
    const defaultImgSrc = `${config.SiteURL}/plugins/${manifest.id}/public/noImage.png`;

    /*************** Rendering start *************************/

    const StyledImgWrapper = styled(Grid)(() => {
        const theme = defaultTheme;
        return {
            [theme.breakpoints.up("xs")]: {
                width: 125 * 0.8,
                height: 160 * 0.8,
            },
            [theme.breakpoints.up("sm")]: {
                width: 125 * 1.5,
                height: 160 * 1.5,
            },
            "& img": {
                maxWidth: "100%",
                maxHeight: "100%",
                float: "left",
            },
        };
    });

    const StyledHeadStatusExpired = styled(Avatar)(() => {
        const theme = defaultTheme;
        return {
            backgroundColor: "red",
            [theme.breakpoints.up("xs")]: {
                width: "2.5rem",
                height: "2.5rem",
            },
            [theme.breakpoints.up("sm")]: {
                width: "2rem",
                height: "2rem",
            },
        };
    });

    const checkExpire = () => {
        if (expireDays === -1) {
            return false;
        }

        function findStatusAndCheck(status: string) {
            const step = FindStatusInWorkflow(status, workflow);
            if (!step?.action_date) {
                return null;
            }
            const action_date = step.action_date;
            const expiredDate = moment(action_date).add(expireDays, "days");
            if (moment(Date.now()) > expiredDate) {
                return true;
            }
            return false;
        }

        const rc = findStatusAndCheck(STATUS_RENEW_CONFIRMED);

        if (rc !== null) return rc;

        const dlv = findStatusAndCheck(STATUS_DELIVIED);

        if (dlv !== null) return dlv;

        return false;
    };

    type distance = {
        active: boolean;
        days?: number;
        message?: string;
        color?: string;
        fontColor?: string;
    };

    const computeDistance: () => distance = () => {
        if (expireDays === -1) {
            return {
                active: false,
            };
        }

        if (currentStep.status === STATUS_RETURNED) {
            return {
                active: false,
            };
        }

        function computeByStatus(status: string) {
            const step = FindStatusInWorkflow(status, workflow);
            if (!step?.action_date) {
                return null;
            }
            const action_date = step.action_date;
            const expiredDate = moment(action_date).add(expireDays, "days");
            const days = expiredDate.diff(moment(Date.now()), "days");
            if (days < 0) {
                return {
                    active: true,
                    days: days,
                    message: Template(TEXT["DISTANCE_AFTER_RETRUN"], [
                        Math.abs(days),
                    ]),
                    color: "red",
                    fontColor: "white",
                };
            }

            if (days < 7) {
                return {
                    active: true,
                    days: days,
                    message: Template(TEXT["DISTANCE_TO_RETRUN"], [
                        Math.abs(days),
                    ]),
                    color: "yellow",
                    fontColor: "black",
                };
            }

            return {
                active: true,
                days: days,
                message: Template(TEXT["DISTANCE_TO_RETRUN"], [Math.abs(days)]),
                color: "yellowgreen",
                fontColor: "black",
            };
        }

        const rc = computeByStatus(STATUS_RENEW_CONFIRMED);

        if (rc !== null) return rc;

        const dlv = computeByStatus(STATUS_DELIVIED);

        if (dlv !== null) return dlv;

        return {
            active: false,
        };
    };

    const distance = computeDistance();

    const StyledHeadStatus = styled(Grid)(() => {
        const theme = defaultTheme;

        return {
            "& .Distance": {
                background: distance.color,
                color: distance.fontColor,
            },
        };
    });

    const distanceStatus = distance.active && (
        <Chip
            size={"small"}
            // variant={"outlined"}
            icon={<AlarmOnIcon />}
            label={distance.message}
            color={"primary"}
            className={"Distance"}
        />
    );

    const headStatus = (
        <StyledHeadStatus container>
            <Grid item>{distanceStatus}</Grid>
        </StyledHeadStatus>
    );

    const canDelete = () => {
        if (
            borrow.role.findIndex(
                (role) => role === "MASTER" || role === "LIBWORKER"
            ) !== -1 &&
            (currentStep.status === STATUS_REQUESTED ||
                currentStep.status === STATUS_CONFIRMED ||
                currentStep.status === STATUS_RETURNED)
        ) {
            return true;
        }

        if (
            borrow.role.findIndex((role) => role === "BORROWER") !== -1 &&
            currentStep.status === STATUS_REQUESTED
        ) {
            return true;
        }

        return false;
    };

    const handleDelete = async () => {
        if (!confirm(TEXT["CONFIRM_DELETE"])) {
            return;
        }

        const request: WorkflowRequest = {
            master_key:
                borrow.role.findIndex((role) => role === "MASTER") !== -1
                    ? post.id
                    : borrow.relations_keys.master,
            act_user: currentUser.username,
            delete: true,
        };
        try {
            setLoading(true);
            const data = await Client4.doFetch<Result>(
                `/plugins/${manifest.id}/workflow`,
                {
                    method: "POST",
                    body: JSON.stringify(request),
                }
            );
            setLoading(false);

            if (data.error) {
                alert(TEXT["ALERT_DELETE_ERROR"] + data.error);
                console.error(data);
                return;
            }
        } catch (e) {
            setLoading(false);
            alert(TEXT["ALERT_DELETE_ERROR"] + e);
            console.error(e);
            return;
        }

        alert(TEXT["ALERT_DELETE_SUCC"]);
    };

    const buttonbar = (
        <Grid container justifyContent={"flex-end"}>
            {canDelete() && (
                <IconButton onClick={handleDelete}>
                    <DeleteIcon />
                </IconButton>
            )}
        </Grid>
    );

    const titleBar = (
        <Grid item container alignItems={"center"}>
            <Grid item xs={8}>
                {headStatus}
            </Grid>
            <Grid item xs={4}>
                {buttonbar}
            </Grid>
        </Grid>
    );

    // <Link href={`${config.SiteURL}/${currentTeam.id}/pl/${borrow.dataOrImage.book_post_id}`}>
    //     {"转到图书"}
    // </Link>

    const BookInfo = styled(Grid)(() => {
        const theme = defaultTheme;
        return {
            "& .BookInfo": {
                [theme.breakpoints.up("xs")]: {
                    marginBottom: "1rem",
                },
                [theme.breakpoints.up("sm")]: {
                    marginBottom: "1.5rem",
                },
            },
            "& .BookName": {
                [theme.breakpoints.up("xs")]: {
                    fontSize: "2rem",
                    fontWeight: "bold",
                    // marginBottom: "1rem",
                },
                [theme.breakpoints.up("sm")]: {
                    fontSize: "3rem",
                    fontWeight: "bold",
                    // marginBottom: "2rem",
                },
            },
            "& .AuthorName": {
                [theme.breakpoints.up("xs")]: {
                    fontSize: "1.2rem",
                    fontWeight: "bold",
                    marginTop: "0.5rem",
                },
                [theme.breakpoints.up("sm")]: {
                    fontSize: "1.5rem",
                    fontWeight: "bold",
                    marginTop: "0.5rem",
                },
            },
            "& .Paticipant": {
                // [theme.breakpoints.up("xs")]: {
                //     marginBottom: "1rem",
                // },
                // [theme.breakpoints.up("sm")]: {
                //     marginBottom: "4rem",
                // },

                "& .PaticipantCommon": {
                    [theme.breakpoints.up("xs")]: {
                        fontSize: "0.8rem",
                    },
                    [theme.breakpoints.up("sm")]: {
                        fontSize: "0.8rem",
                    },
                },

                "& .PaticipantBorrower": {
                    backgroundColor: "purple",
                },
                "& .PaticipantLibworker": {
                    backgroundColor: "green",
                },
                "& .PaticipantKeeper": {
                    backgroundColor: "teal",
                },
            },
        };
    });

    const handleRoleUserClick = (userName: string) => {
        return () => history.push(`/${currentTeam.name}/messages/@${userName}`);
    };

    const participants = (
        <Grid container spacing={1}>
            <Grid item>
                {borrow.dataOrImage.borrower_name && (
                    <Chip
                        size={"medium"}
                        // variant={"outlined"}
                        icon={<BorrowIcon />}
                        color="primary"
                        label={borrow.dataOrImage.borrower_name}
                        className={"PaticipantCommon PaticipantBorrower"}
                        clickable
                        onClick={handleRoleUserClick(
                            borrow.dataOrImage.borrower_user
                        )}
                    />
                )}
            </Grid>
            <Grid item>
                <Chip
                    size={"medium"}
                    // variant={"outlined"}
                    icon={<WorkerIcon />}
                    color="primary"
                    label={borrow.dataOrImage.libworker_name}
                    className={"PaticipantCommon PaticipantLibworker"}
                    clickable
                    onClick={handleRoleUserClick(
                        borrow.dataOrImage.libworker_user
                    )}
                />
            </Grid>
            <Grid item>
                {borrow.dataOrImage.keeper_names?.map((keeper_name, i) => (
                    <Chip
                        size={"medium"}
                        // variant={"outlined"}
                        icon={<HouseIcon />}
                        color="primary"
                        label={keeper_name}
                        className={"PaticipantCommon PaticipantKeeper"}
                        clickable
                        onClick={handleRoleUserClick(
                            borrow.dataOrImage.keeper_users[i]
                        )}
                    />
                ))}
            </Grid>
        </Grid>
    );

    const handleLinkToBook = () =>
        history.push(
            `/${currentTeam.name}/pl/${borrow.dataOrImage.book_post_id}`
        );
    const requestInfo = (
        <BookInfo container direction="column">
            <Grid item className={"BookInfo"}>
                <div className={"BookName"}>
                    {borrow.dataOrImage.book_name}
                    {
                        <IconButton onClick={handleLinkToBook}>
                            <BookIcon />
                        </IconButton>
                    }
                </div>
                <div className={"AuthorName"}>{borrow.dataOrImage.author}</div>
            </Grid>
            <Grid item className={"Paticipant"}>
                {participants}
            </Grid>
        </BookInfo>
    );

    const StyledDateAccordion = styled(Accordion)(() => {
        const theme = defaultTheme;
        return {
            width: "100%",
            "& .MuiAccordionSummary-root": {
                fontSize: "1.2rem",
            },
        };
    });

    type wfCallBack = (s: Step, i: number) => boolean;

    const exploreWF = (cb: wfCallBack) => {
        let passed: { [key: number]: boolean } = {};

        // return bool value of cb
        const next = (step: Step, cb: wfCallBack, index: number) => {
            passed[index] = true;

            if (cb(step, index)) {
                return;
            }

            const nextStepIndex = step.next_step_index;

            if (nextStepIndex === null) {
                return;
            }

            for (let i of nextStepIndex) {
                const nextStep = workflow[i];
                if (
                    step.status === STATUS_RENEW_CONFIRMED &&
                    nextStep.status === STATUS_RENEW_REQUESTED
                ) {
                    return;
                }

                if (passed[i]) continue;

                next(nextStep, cb, i);
            }

            return;
        };

        next(workflow[0], cb, 0);
    };

    const addDates = () => {
        const DateField = styled(TextField)(() => {
            const theme = defaultTheme;
            return {
                "& label,input": {
                    [theme.breakpoints.up("xs")]: {
                        fontSize: "1rem",
                    },
                    [theme.breakpoints.up("sm")]: {
                        fontSize: "1.5rem",
                    },
                },
            };
        });

        let actionDates: React.ReactElement[] = [];

        exploreWF((step) => {
            if (!step.completed) {
                return true;
            }

            actionDates.push(
                <Grid item>
                    <DateField
                        label={TEXT["DATE_" + step.status]}
                        value={moment(step.action_date).format("YYYY-MM-DD")}
                        InputProps={{
                            readOnly: true,
                        }}
                    />
                </Grid>
            );

            return false;
        });

        return actionDates;
    };

    const dateOfRequest = (
        <Grid container>
            <StyledDateAccordion>
                <AccordionSummary
                    expandIcon={<ExpandMoreIcon />}
                    id="request_date"
                >
                    <div>{TEXT["date_accordion"]}</div>
                </AccordionSummary>
                <AccordionDetails>
                    <Grid container spacing={1}>
                        {addDates()}
                    </Grid>
                </AccordionDetails>
            </StyledDateAccordion>
        </Grid>
    );

    const StyledWFStepper = styled(Stepper)(() => {
        return {
            "& svg": {
                fontSize: "2rem",
            },
            "& .MuiStepLabel-label": {
                fontSize: "1.2rem",
            },
        };
    });

    const StyledWFButton = styled(Button)(() => {
        return {
            marginLeft: 10,
            marginTop: 10,
            width: "10rem",
            alignSelf: "flex-end",
        };
    });

    const StyledWFTypeChip = styled(Chip)(() => ({
        width: "30%",
        height: "2rem",
        marginTop: "1.5rem",
        fontSize: "1.2rem",
    }));

    let localWorkflow: Step[] = [];
    let localStepIndex: number = -1;
    let lastIndex = -1;
    let lastIndexes: number[] = [];

    exploreWF((step, index) => {
        if (step.workflow_type !== currentStep.workflow_type) {
            lastIndex = index;
            return false;
        }

        lastIndexes.push(lastIndex);
        localWorkflow.push(step);

        if (step.completed) {
            localStepIndex++;
        }

        lastIndex = index;
        return false;
    });
    // const graidentColor = "linear-gradient( 109.6deg,  rgba(45,116,213,1) 11.2%, rgba(121,137,212,1) 91.2% )"
    // background-image: radial-gradient( circle farthest-corner at 10% 20%,  rgba(14,174,87,1) 0%, rgba(12,116,117,1) 90% );
    const graidentColor =
        "radial-gradient( circle farthest-corner at 10% 20%,  rgba(14,174,87,1) 0%, rgba(12,116,117,1) 90% )";

    const ColorlibConnector = styled(StepConnector)(() => ({
        "&.MuiStepConnector-alternativeLabel": {
            top: 22,
        },
        "&.MuiStepConnector-active": {
            "& $line": {
                backgroundImage: graidentColor,
            },
        },
        "&.MuiStepConnector-completed": {
            "& $line": {
                backgroundImage: graidentColor,
            },
        },
        line: {
            height: 3,
            border: 0,
            backgroundColor: "#eaeaf0",
            borderRadius: 1,
        },
    }));

    // const useColorlibStepIconStyles = makeStyles({
    //     root: {
    //         backgroundColor: "#ccc",
    //         zIndex: 1,
    //         color: "#fff",
    //         width: 50,
    //         height: 50,
    //         display: "flex",
    //         borderRadius: "50%",
    //         justifyContent: "center",
    //         alignItems: "center",
    //     },
    //     active: {
    //         backgroundImage: graidentColor,
    //         boxShadow: "0 4px 10px 0 rgba(0,0,0,.25)",
    //     },
    //     completed: {
    //         backgroundImage: graidentColor,
    //     },
    // });

    const StyledColorLibStepIcon = styled("div")(() => ({
        "&.ColorlibStepIcon-root": {
            backgroundColor: "#ccc",
            zIndex: 1,
            color: "#fff",
            width: 50,
            height: 50,
            display: "flex",
            borderRadius: "50%",
            justifyContent: "center",
            alignItems: "center",
        },
        "&.ColorlibStepIcon-active": {
            backgroundImage: graidentColor,
            boxShadow: "0 4px 10px 0 rgba(0,0,0,.25)",
        },
        "&.ColorlibStepIcon-completed": {
            backgroundImage: graidentColor,
        },
    }));

    //Icon shows the actor who has executed this activity,
    //this is conceptly differenct from the meaning of ActRole in Step, which means who WILL execute next activity
    function ColorlibStepIcon(props: StepIconProps) {
        // const classes = useColorlibStepIconStyles();
        const { active, completed } = props;

        let icons: { [index: string]: React.ReactElement } = {};

        localWorkflow.forEach((step, index) => {
            let iconIndex = index + 1;
            let icon: React.ReactElement = <div />;
            const executedRole =
                lastIndexes[index] === -1
                    ? "BORROWER"
                    : workflow[lastIndexes[index]].actor_role;
            switch (executedRole) {
                case "BORROWER":
                    icon = <BorrowIcon />;
                    break;

                case "LIBWORKER":
                    icon = <WorkerIcon />;
                    break;

                case "KEEPER":
                    icon = <HouseIcon />;
                    break;

                default:
                    break;
            }
            icons[iconIndex] = icon;
        });

        // return (
        //     <div
        //         className={clsx(classes.root, {
        //             [classes.active]: active,
        //             [classes.completed]: completed,
        //         })}
        //     >
        //         {icons[String(props.icon)]}
        //     </div>
        // );
        return (
            <StyledColorLibStepIcon
                className={clsx("ColorlibStepIcon-root", {
                    ["ColorlibStepIcon-active"]: active,
                    ["ColorlibStepIcon-completed"]: completed,
                })}
            >
                {icons[String(props.icon)]}
            </StyledColorLibStepIcon>
        );
    }
    const handleStep = async (nextStepIndex: number, backward: boolean) => {
        const step = workflow[nextStepIndex];

        if (!confirm(TEXT["CONFIRM_STEP"] + TEXT["ST_" + step.status])) {
            return;
        }

        const request: WorkflowRequest = {
            master_key:
                borrow.role.findIndex((role) => role === "MASTER") !== -1
                    ? post.id
                    : borrow.relations_keys.master,
            act_user: currentUser.username,
            next_step_index: nextStepIndex,
            backward: backward,
        };

        try {
            setLoading(true);

            mutil.initAsyncMsg(2, {
                open: true,
                text: TEXT["WORKFLOW_SUCC"],
                serverity: "success",
            });
            const data = await Client4.doFetch<Result>(
                `/plugins/${manifest.id}/workflow`,
                {
                    method: "POST",
                    body: JSON.stringify(request),
                }
            );
            //If process successfully, the following logic won't be executed,
            //because the component is being rendered in the fetching process..
            //So we have to check the state and recover in another rendering's useEffect()
            setLoading(false);

            if (data.error) {
                mutil.setMsgBox({
                    open: true,
                    text: TEXT["WORKFLOW_ERROR"] + data.error,
                    serverity: "error",
                });
                console.error(data);
                mutil.disableAsyncMsg();
                return;
            }
        } catch (e) {
            setLoading(false);

            mutil.setMsgBox({
                open: true,
                text: TEXT["WORKFLOW_ERROR"] + e,
                serverity: "error",
            });
            console.error(e);
            mutil.disableAsyncMsg();
            return;
        }

        mutil.checkAndDisplayAsyncMsg();
    };

    const addStepButtons = () => {
        let btns: React.ReactElement[] = [];

        if (
            currentStep.actor_role === "LIBWORKER" &&
            currentStep.status !== STATUS_REQUESTED &&
            currentStep.status !== STATUS_RETURNED
        ) {
            btns.push(
                <StyledWFButton
                    variant={"contained"}
                    color={"secondary"}
                    onClick={() =>
                        handleStep(currentStep.last_step_index, true)
                    }
                >
                    {TEXT["REJECT"]}
                </StyledWFButton>
            );
        }

        if (currentStep.next_step_index !== null) {
            for (let n of currentStep.next_step_index) {
                if (
                    workflow[n].status === STATUS_RENEW_REQUESTED &&
                    borrow.dataOrImage.renewed_times >= maxRenewTimes
                )
                    continue;

                btns.push(
                    <StyledWFButton
                        variant={"contained"}
                        color={"primary"}
                        onClick={() => handleStep(n, false)}
                    >
                        {TEXT["ST_" + workflow[n].status]}
                    </StyledWFButton>
                );
            }
        }

        return btns;
    };

    const showWfButtons = () => {
        if (
            borrow.role.findIndex((role) => {
                return currentStep.actor_role === role || role === "MASTER";
            }) !== -1
        ) {
            return true;
        }
        return false;
    };

    const borWorkflow = (
        <Grid container direction={"column"}>
            <StyledWFTypeChip
                variant={"outlined"}
                color="primary"
                label={TEXT["WF_" + currentStep.workflow_type]}
            />
            <StyledWFStepper
                alternativeLabel
                activeStep={localStepIndex}
                connector={<ColorlibConnector />}
            >
                {localWorkflow.map((step) => (
                    <Step key={step.status}>
                        <StepLabel StepIconComponent={ColorlibStepIcon}>
                            {TEXT["ST_" + step.status]}
                        </StepLabel>
                    </Step>
                ))}
            </StyledWFStepper>
            {showWfButtons() && (
                <Grid container justifyContent={"flex-end"}>
                    {addStepButtons()}
                </Grid>
            )}
        </Grid>
    );

    const operation = (
        <Grid container direction={"column"} justifyContent={"flex-end"}>
            <Grid item>{dateOfRequest}</Grid>
            <Grid item>{borWorkflow}</Grid>
        </Grid>
    );

    const handleImgError = () => {
        setImgSrcReplace(defaultImgSrc);
    };

    const main = (
        <Grid container spacing={2}>
            <StyledImgWrapper item xs={matchesSm ? 2 : 4}>
                <Image
                    src={imgSrcReplace || imgSrc.current}
                    handleError={handleImgError}
                />
            </StyledImgWrapper>
            <Grid item xs={matchesSm ? 4 : 8}>
                {requestInfo}
            </Grid>
            <Grid item xs={matchesSm ? 6 : 12}>
                {operation}
            </Grid>
        </Grid>
    );

    const StyledPaper = styled(Paper)(() => {
        const theme = defaultTheme;
        return {
            padding: theme.spacing(2),
            margin: "auto",
            maxWidth: "100%",
            position: "relative",
        };
    });

    return (
        <StylesProvider injectFirst>
            <StyledPaper>
                <Grid container direction={"column"}>
                    <Grid item>{titleBar}</Grid>
                    <Grid item>{main}</Grid>
                </Grid>
            </StyledPaper>
            {loading && <InProgress open={loading} />}
            <MsgBox {...mutil.msgBox} close={mutil.onCloseMsg} />
        </StylesProvider>
    );
}

BorrowType.propTypes = {
    post: PropTypes.object.isRequired,
    theme: PropTypes.object.isRequired,
};

export default React.memo(
    BorrowType,
    (prev, next) => prev.post.message === next.post.message
);
